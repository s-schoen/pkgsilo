package api

import (
	"errors"
	"net/http"

	"pkgsilo/core/authz"
	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	"pkgsilo/feature/group/service"
	identityapi "pkgsilo/feature/identity/api"
)

// Problem types of the group endpoints.
var (
	groupNotFoundType  = envelope.DefineProblemType("group-not-found", "Group not found", http.StatusNotFound)
	groupNameTakenType = envelope.DefineProblemType("group-name-taken", "Group name taken", http.StatusConflict)
	memberNotFoundType = envelope.DefineProblemType("member-not-found", "Member not found", http.StatusNotFound)
)

var (
	groupNotFoundProblem  = groupNotFoundType.New(envelope.Occurrence{Detail: "The group does not exist."})
	groupNameTakenProblem = groupNameTakenType.New(envelope.Occurrence{Detail: "The group name is already taken."})
	memberNotFoundProblem = memberNotFoundType.New(envelope.Occurrence{
		Detail: "The identity is not a member of the group.",
	})
	forbiddenProblem = envelope.ForbiddenProblem.New(envelope.Occurrence{
		Detail: "The policies do not allow this action on the group.",
	})
)

// writeError answers a service error with its problem. Unexpected errors are logged and
// answered with 500, without exposing their details.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var fieldsDenied *authz.FieldsDeniedError
	switch {
	case errors.As(err, &fieldsDenied):
		envelope.WriteProblem(w, request.FieldsDeniedProblem(fieldsDenied.Fields...))
	case errors.Is(err, authz.ErrDenied):
		envelope.WriteProblem(w, forbiddenProblem)
	case errors.Is(err, service.ErrGroupNotFound):
		envelope.WriteProblem(w, groupNotFoundProblem)
	case errors.Is(err, service.ErrGroupNameTaken):
		envelope.WriteProblem(w, groupNameTakenProblem)
	case errors.Is(err, service.ErrIdentityNotFound):
		// The problem of the identity endpoints, so that clients recognize a missing identity in one way.
		envelope.WriteProblem(w, identityapi.IdentityNotFoundProblem)
	case errors.Is(err, service.ErrMemberNotFound):
		envelope.WriteProblem(w, memberNotFoundProblem)
	default:
		h.logger.ErrorContext(r.Context(), "group request failed", "error", err)
		envelope.WriteProblem(w, envelope.InternalServerErrorProblem.New(envelope.Occurrence{}))
	}
}
