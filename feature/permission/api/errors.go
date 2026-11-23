package api

import (
	"errors"
	"net/http"

	"pkgsilo/core/authz"
	"pkgsilo/core/envelope"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/feature/permission/service"
)

var forbiddenProblem = envelope.ForbiddenProblem.New(envelope.Occurrence{
	Detail: "Only the caller's own permissions can be listed.",
})

// writeError answers a service error with its problem. Unexpected errors are logged and
// answered with 500, without exposing their details.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authz.ErrDenied):
		envelope.WriteProblem(w, forbiddenProblem)
	case errors.Is(err, service.ErrIdentityNotFound):
		// The problem of the identity endpoints, so that clients recognize a missing identity in one way.
		envelope.WriteProblem(w, identityapi.IdentityNotFoundProblem)
	default:
		h.logger.ErrorContext(r.Context(), "permission request failed", "error", err)
		envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))
	}
}
