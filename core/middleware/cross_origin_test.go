package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/envelope"
	"pkgsilo/core/middleware"
)

func TestCrossOriginProtection(t *testing.T) {
	t.Parallel()
	h := middleware.CrossOriginProtection(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	tests := []struct {
		name   string
		method string
		header map[string]string
		reject bool
	}{
		{
			name: "cross-site write", method: http.MethodPost, header: map[string]string{"Sec-Fetch-Site": "cross-site"},
			reject: true,
		},
		{
			name: "same-site write", method: http.MethodPut, header: map[string]string{"Sec-Fetch-Site": "same-site"},
			reject: true,
		},
		{
			name: "same-origin write", method: http.MethodPost,
			header: map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://example.com"},
		},
		{name: "user-initiated write", method: http.MethodPatch, header: map[string]string{"Sec-Fetch-Site": "none"}},
		{
			name: "mismatched Origin without Fetch Metadata", method: http.MethodDelete,
			header: map[string]string{"Origin": "https://attacker.example"}, reject: true,
		},
		{
			name: "matching Origin without Fetch Metadata", method: http.MethodDelete,
			header: map[string]string{"Origin": "http://example.com"},
		},
		{name: "write without either header", method: http.MethodPost},
		{name: "cross-site read", method: http.MethodGet, header: map[string]string{"Sec-Fetch-Site": "cross-site"}},
		{name: "cross-site HEAD", method: http.MethodHead, header: map[string]string{"Sec-Fetch-Site": "cross-site"}},
		{
			name: "cross-site OPTIONS", method: http.MethodOptions,
			header: map[string]string{"Sec-Fetch-Site": "cross-site"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tt.method, "http://example.com/", nil)
			for name, value := range tt.header {
				req.Header.Set(name, value)
			}

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if !tt.reject {
				assert.Equal(t, http.StatusNoContent, rec.Code)

				return
			}
			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
			assert.JSONEq(t, `{"type":"https://pkgsilo/problems/cross-origin-request","title":"Cross-origin request",`+
				`"status":403,"detail":"The API accepts state-changing requests from browsers only from its own origin."}`,
				rec.Body.String())
		})
	}
}
