package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveRequestID runs a request through RequestID and returns the ID seen by the handler.
func serveRequestID(t *testing.T, req *http.Request) (string, *httptest.ResponseRecorder) {
	t.Helper()

	var ctxID string
	h := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctxID = RequestIDFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return ctxID, rec
}

func TestRequestID_ReusesValidIncomingID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "client-id_1.0")

	ctxID, rec := serveRequestID(t, req)

	assert.Equal(t, "client-id_1.0", ctxID)
	assert.Equal(t, "client-id_1.0", rec.Header().Get(RequestIDHeader))
}

func TestRequestID_GeneratesUUIDv7WhenMissing(t *testing.T) {
	ctxID, rec := serveRequestID(t, httptest.NewRequest(http.MethodGet, "/", nil))

	id, err := uuid.Parse(ctxID)
	require.NoError(t, err)
	assert.Equal(t, 7, int(id[6]>>4), "UUID version")
	assert.Equal(t, ctxID, rec.Header().Get(RequestIDHeader))

	otherID, _ := serveRequestID(t, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.NotEqual(t, ctxID, otherID)
}

func TestRequestID_ReplacesInvalidIncomingID(t *testing.T) {
	tests := map[string]string{
		"too long":          strings.Repeat("a", 129),
		"whitespace":        "abc def",
		"special character": `abc"}`,
	}

	for name, incoming := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(RequestIDHeader, incoming)

			ctxID, rec := serveRequestID(t, req)

			assert.NotEqual(t, incoming, ctxID)
			_, err := uuid.Parse(ctxID)
			require.NoError(t, err)
			assert.Equal(t, ctxID, rec.Header().Get(RequestIDHeader))
		})
	}
}

func TestRequestIDFromContext_Empty(t *testing.T) {
	assert.Empty(t, RequestIDFromContext(t.Context()))
}
