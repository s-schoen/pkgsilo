package logger_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/logger/loggertest"
	"pkgsilo/core/middleware"
)

// requestContext returns the context middleware.RequestID gives a request carrying id.
func requestContext(t *testing.T, id string) context.Context {
	t.Helper()

	var handled *http.Request
	h := middleware.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		handled = r
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, id)
	h.ServeHTTP(httptest.NewRecorder(), req)

	return handled.Context()
}

func TestLogger_AddsRequestIDFromContext(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	ctx := requestContext(t, "req-1")

	logger.InfoContext(ctx, "plain")
	logger.With("component", "test").InfoContext(ctx, "derived via With")
	logger.WithGroup("grp").InfoContext(ctx, "derived via WithGroup")
	logger.InfoContext(t.Context(), "no request")

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 4)

	assert.Equal(t, "req-1", entries[0]["request_id"])
	assert.Equal(t, "req-1", entries[1]["request_id"])
	assert.Equal(t, map[string]any{"request_id": "req-1"}, entries[2]["grp"])
	assert.NotContains(t, entries[3], "request_id")
}
