package api

import (
	"errors"
	"fmt"
	"net/http"

	"pkgsilo/core/authn"
	"pkgsilo/core/envelope"
	"pkgsilo/feature/session/service"
)

// sessionNotFoundType is the problem type of a request for the current session that isn't authenticated with
// one, such as one with an API key.
var sessionNotFoundType = envelope.DefineProblemType("session-not-found", "Session not found", http.StatusNotFound)

var (
	sessionNotFoundProblem = sessionNotFoundType.New(envelope.Occurrence{
		Detail: "The request is not authenticated with a session.",
	})
	// unauthorizedProblem answers every rejected login, whatever the reason, like authn.Require answers a
	// rejected credential.
	unauthorizedProblem = envelope.UnauthorizedProblem.New(envelope.Occurrence{})
)

// toAuthnError returns the error of a login for authn.LoginRecorder: a rejection wraps the authn reason error
// that fits, and every other error stays as it is.
func toAuthnError(err error) error {
	switch {
	case errors.Is(err, service.ErrUnknownCredentials):
		return fmt.Errorf("%w: %w", authn.ErrUnknownCredentials, err)
	case errors.Is(err, service.ErrInvalidPassword):
		return fmt.Errorf("%w: %w", authn.ErrInvalidCredentials, err)
	default:
		return err
	}
}

// writeError answers a service error with its problem. Unexpected errors are logged and answered with 500,
// without exposing their details.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrSessionNotFound):
		envelope.WriteProblem(w, sessionNotFoundProblem)
	default:
		h.logger.ErrorContext(r.Context(), "session request failed", "error", err)
		envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))
	}
}
