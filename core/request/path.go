package request

import (
	"net/http"
	"uuid"

	"github.com/go-chi/chi/v5"

	"pkgsilo/core/envelope"
)

// UUIDParam returns the UUID in the path parameter name. A value that isn't a UUID can't identify
// a resource, so it's answered like an unknown one: UUIDParam writes notFound and returns false.
func UUIDParam(w http.ResponseWriter, r *http.Request, name string, notFound envelope.Problem) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		envelope.WriteProblem(w, notFound)
		return uuid.UUID{}, false
	}

	return id, true
}
