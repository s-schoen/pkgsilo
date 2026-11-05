package api

import (
	"errors"
	"net/http"

	"pkgsilo/core/envelope"
	"pkgsilo/feature/apikey/service"
	identityapi "pkgsilo/feature/identity/api"
)

// apiKeyNotFoundType is the problem type of a request for an API key that doesn't exist.
var apiKeyNotFoundType = envelope.DefineProblemType("api-key-not-found", "API key not found", http.StatusNotFound)

var apiKeyNotFoundProblem = apiKeyNotFoundType.New(envelope.Occurrence{
	Detail: "The identity has no API key with this ID.",
})

// writeError answers a service error with its problem. Unexpected errors are logged and
// answered with 500, without exposing their details.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrAPIKeyNotFound):
		envelope.WriteProblem(w, apiKeyNotFoundProblem)
	case errors.Is(err, service.ErrIdentityNotFound):
		// The problem of the identity endpoints, so that clients recognize a missing identity in one way.
		envelope.WriteProblem(w, identityapi.IdentityNotFoundProblem)
	default:
		h.logger.ErrorContext(r.Context(), "api key request failed", "error", err)
		envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))
	}
}
