package request

import (
	"net/http"
	"uuid"

	"pkgsilo/core/authn"
	"pkgsilo/core/envelope"
)

// IdentityQueryParam returns the identity ID in the optional query parameter name, which is a UUID or Me, the ID
// of the actor (see authn.Actor), or nil if the request leaves it out. Any other value is answered with
// InvalidQueryProblem: IdentityQueryParam writes it and returns false. The route must be behind authn.Require.
func IdentityQueryParam(w http.ResponseWriter, r *http.Request, name string) (*uuid.UUID, bool) {
	query := r.URL.Query()
	if !query.Has(name) {
		return nil, true
	}

	value := query.Get(name)
	if value == Me {
		actor := authn.Actor(r)
		return &actor, true
	}

	id, err := uuid.Parse(value)
	if err != nil {
		envelope.WriteProblem(w, InvalidQueryProblem(envelope.ValidationError{
			Parameter: name, Detail: `must be a UUID or "` + Me + `"`,
		}))
		return nil, false
	}

	return &id, true
}
