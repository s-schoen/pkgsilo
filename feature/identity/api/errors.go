package api

import (
	"errors"
	"net/http"

	"pkgsilo/core/authz"
	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
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

var forbiddenProblem = envelope.ForbiddenProblem.New(envelope.Occurrence{
	Detail: "The policies do not allow this action on the identity.",
})

// writeError answers a service error with its problem. Unexpected errors are logged and
// answered with 500, without exposing their details.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	if fieldsDenied, ok := errors.AsType[*authz.FieldsDeniedError](err); ok {
		envelope.WriteProblem(w, request.FieldsDeniedProblem(fieldsDenied.Fields...))
		return
	}

	switch {
	case errors.Is(err, authz.ErrDenied):
		envelope.WriteProblem(w, forbiddenProblem)
	case errors.Is(err, service.ErrIdentityNotFound):
		envelope.WriteProblem(w, IdentityNotFoundProblem)
	case errors.Is(err, service.ErrUsernameTaken):
		envelope.WriteProblem(w, usernameTakenProblem)
	default:
		h.logger.ErrorContext(r.Context(), "identity request failed", "error", err)
		envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))
	}
}
