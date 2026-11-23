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
	"pkgsilo/core/envelope"
)

func TestUUIDParam(t *testing.T) {
	t.Parallel()
	notFound := envelope.NotFoundProblem.New(envelope.Occurrence{Detail: "The thing does not exist."})
	id := uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4")
	tests := []struct {
		name   string
		target string
		wantID uuid.UUID
		wantOK bool
	}{
		{name: "UUID", target: "/things/" + id.String(), wantID: id, wantOK: true},
		{name: "not a UUID", target: "/things/not-a-uuid"},
		{name: "UUID with suffix", target: "/things/" + id.String() + "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var (
				gotID uuid.UUID
				gotOK bool
			)
			router := chi.NewRouter()
			router.Get("/things/{id}", func(w http.ResponseWriter, r *http.Request) {
				gotID, gotOK = UUIDParam(w, r, "id", notFound)
			})
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))

			assert.Equal(t, tt.wantOK, gotOK)
			assert.Equal(t, tt.wantID, gotID)
			if tt.wantOK {
				assert.Zero(t, rec.Body.Len(), "nothing must be written on success")
				return
			}
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.JSONEq(t, `{"type":"about:blank","title":"Not Found","status":404,
				"detail":"The thing does not exist."}`, rec.Body.String())
		})
	}
}

// actorAuthenticator authenticates every request as actor.
type actorAuthenticator struct{}

var actor = uuid.MustParse("1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d")

func (actorAuthenticator) Method() string { return "test" }

func (actorAuthenticator) Authenticate(*http.Request) (authn.Principal, error) {
	return authn.Principal{IdentityID: actor}, nil
}

func (actorAuthenticator) Challenge() string { return "Test" }

func TestIdentityParam(t *testing.T) {
	t.Parallel()
	notFound := envelope.NotFoundProblem.New(envelope.Occurrence{Detail: "The identity does not exist."})
	id := uuid.MustParse("0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4")
	tests := []struct {
		name   string
		target string
		wantID uuid.UUID
		wantOK bool
	}{
		{name: "UUID", target: "/identities/" + id.String(), wantID: id, wantOK: true},
		{name: "me", target: "/identities/me", wantID: actor, wantOK: true},
		{name: "Me in another case", target: "/identities/Me"},
		{name: "not a UUID", target: "/identities/not-a-uuid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var (
				gotID uuid.UUID
				gotOK bool
			)
			logger := slog.New(slog.DiscardHandler)
			meterProvider, tracerProvider := noop.NewMeterProvider(), tracenoop.NewTracerProvider()
			router := chi.NewRouter()
			router.With(authn.Require(logger, meterProvider, tracerProvider, actorAuthenticator{})).
				Get("/identities/{id}", func(w http.ResponseWriter, r *http.Request) {
					gotID, gotOK = IdentityParam(w, r, "id", notFound)
				})
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))

			assert.Equal(t, tt.wantOK, gotOK)
			assert.Equal(t, tt.wantID, gotID)
			if tt.wantOK {
				assert.Zero(t, rec.Body.Len(), "nothing must be written on success")
				return
			}
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.JSONEq(t, `{"type":"about:blank","title":"Not Found","status":404,
				"detail":"The identity does not exist."}`, rec.Body.String())
		})
	}
}
