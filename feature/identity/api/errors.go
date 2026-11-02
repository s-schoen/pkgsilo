package api

import (
	"errors"
	"net/http"

	"pkgsilo/core/envelope"
	"pkgsilo/feature/identity/service"
)

// Problem types of the identity endpoints.
var (
	identityNotFoundType = envelope.DefineProblemType("identity-not-found", "Identity not found", http.StatusNotFound)
	usernameTakenType    = envelope.DefineProblemType("username-taken", "Username taken", http.StatusConflict)
)

// IdentityNotFoundProblem answers a request for an identity that doesn't exist. Other features
// answer a reference to a missing identity with it too, so that clients recognize a missing
// identity in one way.
var IdentityNotFoundProblem = identityNotFoundType.New(envelope.Occurrence{Detail: "The identity does not exist."})

var usernameTakenProblem = usernameTakenType.New(envelope.Occurrence{Detail: "The username is already taken."})

// writeError answers a service error with its problem. Unexpected errors are logged and
// answered with 500, without exposing their details.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrIdentityNotFound):
		envelope.WriteProblem(w, IdentityNotFoundProblem)
	case errors.Is(err, service.ErrUsernameTaken):
		envelope.WriteProblem(w, usernameTakenProblem)
	default:
		h.logger.ErrorContext(r.Context(), "identity request failed", "error", err)
		envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))
	}
}
