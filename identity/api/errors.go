package api

import (
	"errors"
	"net/http"

	"pkgsilo/envelope"
	"pkgsilo/identity/service"
)

var (
	identityNotFoundProblem = envelope.NotFoundProblem.New(envelope.Occurrence{Detail: "The identity does not exist."})
	usernameTakenProblem    = envelope.ConflictProblem.New(envelope.Occurrence{Detail: "The username is already taken."})
)

// writeError answers a service error with its problem. Unexpected errors are logged and
// answered with 500, without exposing their details.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrIdentityNotFound):
		envelope.WriteProblem(w, identityNotFoundProblem)
	case errors.Is(err, service.ErrUsernameTaken):
		envelope.WriteProblem(w, usernameTakenProblem)
	default:
		h.logger.ErrorContext(r.Context(), "identity request failed", "error", err)
		envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))
	}
}
