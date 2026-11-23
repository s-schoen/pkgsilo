// Package tools offers the permission tool of the MCP endpoint, which tells an agent what the policies may
// allow it to do, before it has an object in hand.
package tools

import (
	"uuid"

	"pkgsilo/core/envelope"
	"pkgsilo/feature/permission/service"
)

// PermissionListInput is the input of permission_list.
type PermissionListInput struct {
	// Owner is nil if the call leaves it out.
	Owner *uuid.UUID `json:"owner,omitempty" jsonschema:"The ID of an identity, as identity_list and identity_get return it, to list exactly the actions on the resources that it owns instead: creating its API keys, and every action on its password. Leave it out to list the actions on every type of resource."`
}

// Validate reports no error: any identity ID can be looked up.
func (PermissionListInput) Validate() error {
	return nil
}

// PermissionsOutput is the output of permission_list, as PermissionsResponse is in the HTTP API. The members'
// descriptions are part of the output schema, which describes them to agents.
type PermissionsOutput struct {
	ID             uuid.UUID                `json:"id" jsonschema:"The ID of your identity."`
	AllowedActions envelope.Field[[]string] `json:"allowedActions,omitzero" jsonschema:"The names of the actions that the policies may allow you to do, such as identity:delete or identity:create:username. Without owner, an action is listed if it may be allowed on some resource of its type: an object's own allowedActions tell exactly. With owner, the actions on the resources that the owner owns, such as apikey:create:name or password:reset, exactly."`
}

func toPermissionsOutput(p service.Permissions) PermissionsOutput {
	return PermissionsOutput{ID: p.ID, AllowedActions: envelope.Visible([]string(p.AllowedActions))}
}
