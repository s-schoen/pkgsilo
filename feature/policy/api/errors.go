package api

import (
	"errors"
	"net/http"

	"pkgsilo/core/authz"
	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	"pkgsilo/feature/policy/service"
)

// Problem types of the policy endpoints.
var (
	policyNotFoundType  = envelope.DefineProblemType("policy-not-found", "Policy not found", http.StatusNotFound)
	policyNameTakenType = envelope.DefineProblemType("policy-name-taken", "Policy name taken", http.StatusConflict)
	invalidPolicyType   = envelope.DefineProblemType("invalid-policy", "Invalid policy", http.StatusUnprocessableEntity)
)

var (
	policyNotFoundProblem  = policyNotFoundType.New(envelope.Occurrence{Detail: "The policy does not exist."})
	policyNameTakenProblem = policyNameTakenType.New(envelope.Occurrence{
		Detail: "The policy name is already taken.",
	})
	forbiddenProblem = envelope.ForbiddenProblem.New(envelope.Occurrence{
		Detail: "The policies do not allow this action on the policy.",
	})
)

// invalidPolicyProblem answers a policy that can't be saved, explaining why in the detail.
func invalidPolicyProblem(reason string) envelope.Problem {
	return invalidPolicyType.New(envelope.Occurrence{Detail: "The policy is invalid: " + reason})
}

// writeError answers a service error with its problem. Unexpected errors are logged and
// answered with 500, without exposing their details.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		fieldsDenied *authz.FieldsDeniedError
		invalid      *service.InvalidPolicyError
	)
	switch {
	case errors.As(err, &fieldsDenied):
		envelope.WriteProblem(w, request.FieldsDeniedProblem(fieldsDenied.Fields...))
	case errors.Is(err, authz.ErrDenied):
		envelope.WriteProblem(w, forbiddenProblem)
	case errors.As(err, &invalid):
		envelope.WriteProblem(w, invalidPolicyProblem(invalid.Reason))
	case errors.Is(err, service.ErrPolicyNotFound):
		envelope.WriteProblem(w, policyNotFoundProblem)
	case errors.Is(err, service.ErrPolicyNameTaken):
		envelope.WriteProblem(w, policyNameTakenProblem)
	default:
		h.logger.ErrorContext(r.Context(), "policy request failed", "error", err)
		envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))
	}
}
