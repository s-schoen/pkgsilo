package request

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

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
