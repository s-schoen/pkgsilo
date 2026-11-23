package api

import (
	"uuid"

	"pkgsilo/core/envelope"
	"pkgsilo/feature/permission/service"
)

// PermissionsResponse is the representation of the caller's permissions in responses.
type PermissionsResponse struct {
	ID             uuid.UUID                `json:"id" jsonschema:"The ID of the caller's identity."`
	AllowedActions envelope.Field[[]string] `json:"allowedActions,omitzero" jsonschema:"The names of the actions that the policies may allow the caller to do, such as identity:delete or identity:create:username. Without the owner parameter, an action is listed if it may be allowed on some resource of its type, erring only toward listing it: an object's own allowedActions tell exactly. With it, the actions on the resources that the owner owns, such as apikey:create:name or password:reset, exactly."`
}

func toPermissionsResponse(p service.Permissions) PermissionsResponse {
	return PermissionsResponse{ID: p.ID, AllowedActions: envelope.Visible([]string(p.AllowedActions))}
}
