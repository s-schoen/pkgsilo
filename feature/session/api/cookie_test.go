package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/authn"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
)

// TestExpireCookieOf checks that the session cookie is deleted only for a request that's authenticated with a
// session of the identity whose sessions ended.
func TestExpireCookieOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		method     string
		identityID uuid.UUID
		want       []string
	}{
		{name: "own session", method: method, identityID: apitest.Actor, want: []string{expiredCookieHeader}},
		{name: "another identity's sessions", method: method, identityID: uuid.NewV4()},
		{name: "API key", method: "api_key", identityID: apitest.Actor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authenticate := authn.Require(loggertest.Discard(), noop.NewMeterProvider(),
				tracenoop.NewTracerProvider(), stubAuthenticator{method: tt.method})
			handler := authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ExpireCookieOf(w, r, tt.identityID)
				w.WriteHeader(http.StatusNoContent)
			}))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", nil))

			assert.Equal(t, http.StatusNoContent, rec.Code)
			assert.Equal(t, tt.want, rec.Header().Values("Set-Cookie"))
		})
	}
}
