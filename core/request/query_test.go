package request

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/authn"
)

func TestIdentityQueryParam(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4")
	tests := []struct {
		name   string
		target string
		wantID *uuid.UUID
		wantOK bool
	}{
		{name: "absent", target: "/things", wantOK: true},
		{name: "UUID", target: "/things?owner=" + id.String(), wantID: &id, wantOK: true},
		{name: "me", target: "/things?owner=me", wantID: &actor, wantOK: true},
		{name: "Me in another case", target: "/things?owner=Me"},
		{name: "empty", target: "/things?owner="},
		{name: "not a UUID", target: "/things?owner=not-a-uuid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var (
				gotID *uuid.UUID
				gotOK bool
			)
			logger := slog.New(slog.DiscardHandler)
			meterProvider, tracerProvider := noop.NewMeterProvider(), tracenoop.NewTracerProvider()
			router := chi.NewRouter()
			router.With(authn.Require(logger, meterProvider, tracerProvider, actorAuthenticator{})).
				Get("/things", func(w http.ResponseWriter, r *http.Request) {
					gotID, gotOK = IdentityQueryParam(w, r, "owner")
				})
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))

			assert.Equal(t, tt.wantOK, gotOK)
			assert.Equal(t, tt.wantID, gotID)
			if tt.wantOK {
				assert.Zero(t, rec.Body.Len(), "nothing must be written on success")
				return
			}
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.JSONEq(t, `{"type":"https://pkgsilo/problems/invalid-query","title":"Invalid query parameters",
				"status":400,"detail":"The request contains invalid query parameters.",
				"errors":[{"parameter":"owner","detail":"must be a UUID or \"me\""}]}`, rec.Body.String())
		})
	}
}
