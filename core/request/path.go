package request

import (
	"net/http"
	"uuid"

	"github.com/go-chi/chi/v5"

	"pkgsilo/core/authn"
	"pkgsilo/core/envelope"
)

// Me is the value of a path parameter that names an identity which stands for the actor's own identity.
const Me = "me"

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

// IdentityParam returns the identity ID in the path parameter name, which is a UUID or Me, the ID of the
// actor (see authn.Actor). Any other value is answered like UUIDParam answers it. The route must be behind
// authn.Require.
func IdentityParam(w http.ResponseWriter, r *http.Request, name string, notFound envelope.Problem) (uuid.UUID, bool) {
	if chi.URLParam(r, name) == Me {
		return authn.Actor(r), true
	}

	return UUIDParam(w, r, name, notFound)
}
