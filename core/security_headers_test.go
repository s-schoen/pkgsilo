package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	want := map[string]string{
		"X-Content-Type-Options":            "nosniff",
		"X-Frame-Options":                   "DENY",
		"Content-Security-Policy":           "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'",
		"Referrer-Policy":                   "no-referrer",
		"X-Permitted-Cross-Domain-Policies": "none",
	}
	for header, value := range want {
		assert.Equal(t, []string{value}, rec.Header().Values(header), header)
	}
}
