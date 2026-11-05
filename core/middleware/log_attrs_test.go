package middleware_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/middleware"
	"pkgsilo/test/loggertest"
)

func TestAddLogAttrs_ReachesLaterRecordsOfTheRequest(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	h := middleware.RequestLogger(logger)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		logger.InfoContext(r.Context(), "before")
		middleware.AddLogAttrs(r.Context(), slog.String("identity_id", "ada"))
		logger.InfoContext(r.Context(), "after")
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 3)
	assert.NotContains(t, entries[0], "identity_id")
	assert.Equal(t, "ada", entries[1]["identity_id"])
	assert.Equal(t, "request", entries[2]["msg"])
	assert.Equal(t, "ada", entries[2]["identity_id"], "RequestLogger's own record has the attribute")
}

func TestAddLogAttrs_DoesNothingOutsideRequestLogger(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	middleware.AddLogAttrs(ctx, slog.String("identity_id", "ada"))

	assert.Nil(t, middleware.LogAttrsFromContext(ctx))
}
