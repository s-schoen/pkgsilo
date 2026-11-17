package api

import (
	"errors"
	"net/http"

	"pkgsilo/core/authz"
	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/feature/password/service"
)

// passwordNotFoundType is the problem type of a request for the password of an identity that has none.
var passwordNotFoundType = envelope.DefineProblemType("password-not-found", "Password not found", http.StatusNotFound)

// currentPasswordPointer points to the member of the request body that holds the current password.
const currentPasswordPointer = "#/currentPassword"

var (
	passwordNotFoundProblem = passwordNotFoundType.New(envelope.Occurrence{
		Detail: "The identity has no password.",
	})
	forbiddenProblem = envelope.ForbiddenProblem.New(envelope.Occurrence{
		Detail: "The policies do not allow this action on the password.",
	})
	// currentPasswordRequiredProblem and wrongCurrentPasswordProblem tell their cases apart, which is safe
	// for a caller who's authenticated already.
	currentPasswordRequiredProblem = request.InvalidBodyProblem(envelope.ValidationError{
		Pointer: currentPasswordPointer, Detail: "is required to change the password",
	})
	wrongCurrentPasswordProblem = request.InvalidBodyProblem(envelope.ValidationError{
		Pointer: currentPasswordPointer, Detail: "does not match the current password",
	})
)

// writeError answers a service error with its problem. Unexpected errors are logged and
// answered with 500, without exposing their details.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authz.ErrDenied):
		envelope.WriteProblem(w, forbiddenProblem)
	case errors.Is(err, service.ErrPasswordNotFound):
		envelope.WriteProblem(w, passwordNotFoundProblem)
	case errors.Is(err, service.ErrIdentityNotFound):
		// The problem of the identity endpoints, so that clients recognize a missing identity in one way.
		envelope.WriteProblem(w, identityapi.IdentityNotFoundProblem)
	case errors.Is(err, service.ErrCurrentPasswordRequired):
		envelope.WriteProblem(w, currentPasswordRequiredProblem)
	case errors.Is(err, service.ErrWrongCurrentPassword):
		envelope.WriteProblem(w, wrongCurrentPasswordProblem)
	default:
		h.logger.ErrorContext(r.Context(), "password request failed", "error", err)
		envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))
	}
}
