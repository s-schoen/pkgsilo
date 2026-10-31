package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/middleware"
)

// serveRequestID runs a request through middleware.RequestID and returns the ID seen by the handler.
func serveRequestID(t *testing.T, req *http.Request) (string, *httptest.ResponseRecorder) {
	t.Helper()

	var ctxID string
	h := middleware.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctxID = middleware.RequestIDFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return ctxID, rec
}

func TestRequestID_ReusesValidIncomingID(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.RequestIDHeader, "client-id_1.0")

	ctxID, rec := serveRequestID(t, req)

	assert.Equal(t, "client-id_1.0", ctxID)
	assert.Equal(t, "client-id_1.0", rec.Header().Get(middleware.RequestIDHeader))
}

func TestRequestID_GeneratesUUIDv7WhenMissing(t *testing.T) {
	t.Parallel()
	ctxID, rec := serveRequestID(t, httptest.NewRequest(http.MethodGet, "/", nil))

	id, err := uuid.Parse(ctxID)
	require.NoError(t, err)
	assert.Equal(t, 7, int(id[6]>>4), "UUID version")
	assert.Equal(t, ctxID, rec.Header().Get(middleware.RequestIDHeader))

	otherID, _ := serveRequestID(t, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.NotEqual(t, ctxID, otherID)
}

func TestRequestID_ReplacesInvalidIncomingID(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"too long":          strings.Repeat("a", 129),
		"whitespace":        "abc def",
		"special character": `abc"}`,
	}

	for name, incoming := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(middleware.RequestIDHeader, incoming)

			ctxID, rec := serveRequestID(t, req)

			assert.NotEqual(t, incoming, ctxID)
			_, err := uuid.Parse(ctxID)
			require.NoError(t, err)
			assert.Equal(t, ctxID, rec.Header().Get(middleware.RequestIDHeader))
		})
	}
}

func TestRequestIDFromContext_Empty(t *testing.T) {
	t.Parallel()
	assert.Empty(t, middleware.RequestIDFromContext(t.Context()))
}
