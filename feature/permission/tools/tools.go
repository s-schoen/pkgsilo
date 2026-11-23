package tools

import (
	"context"
	"uuid"

	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/permission/service"
)

// Tools offers the permission tools.
type Tools struct {
	service service.PermissionService
}

var _ mcpserver.Toolset = (*Tools)(nil)

// New returns the permission tools, which list permissions through svc.
func New(svc service.PermissionService) *Tools {
	return &Tools{service: svc}
}

// Register adds the permission tools to reg.
func (t *Tools) Register(reg *mcpserver.Registry) {
	reg.Add(mcpserver.Tool{
		Name: "permission_list",
		Description: "Lists the actions that the policies of pkgsilo may allow you to do, such as " +
			"identity:create:username or group:addMember, to find out what you can do before you try it. " +
			"Without owner, a listed action is allowed on some resource of its type, but maybe not on every one: " +
			"the allowedActions of an object, as the get and list tools return it, tell exactly what you may do " +
			"to it. An action that isn't listed is denied on every resource. With owner, it lists exactly the " +
			"actions on the resources that the identity owns: creating its API keys, and every action on its " +
			"password. The list is only a guide: every call is still checked on its own. " +
			"Fails with identity_not_found if the owner does not exist, or if the policies do not allow you to " +
			"read it: the two cases look the same.",
		Kind: mcpserver.Read,
	}, t.list)
}

func (t *Tools) list(ctx context.Context, actor uuid.UUID, in PermissionListInput) (PermissionsOutput, error) {
	permissions, err := t.service.List(ctx, actor, actor, in.Owner)
	if err != nil {
		return PermissionsOutput{}, translateError(err)
	}

	return toPermissionsOutput(permissions), nil
}
