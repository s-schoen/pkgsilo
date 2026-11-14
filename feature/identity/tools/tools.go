package tools

import (
	"context"
	"uuid"

	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/identity/service"
)

// Tools offers the identity tools.
type Tools struct {
	service service.IdentityService
}

var _ mcpserver.Toolset = (*Tools)(nil)

// New returns the identity tools, which manage identities through svc.
func New(svc service.IdentityService) *Tools {
	return &Tools{service: svc}
}

// Register adds the identity tools to reg.
func (t *Tools) Register(reg *mcpserver.Registry) {
	reg.Add(mcpserver.Tool{
		Name: "identity_whoami",
		Description: "Returns the identity that you act as: the user of pkgsilo whose API key authenticates you. " +
			"Every other tool acts as this identity, and the policies of pkgsilo decide what it may do. " +
			"Fields that the policies do not allow you to read are absent. createdBy and updatedBy are the nil " +
			"UUID for changes that pkgsilo made itself, such as creating the first identity. " +
			"Fails with identity_not_found if the policies do not allow you to read your own identity.",
		Kind: mcpserver.Read,
	}, t.whoami)
}

func (t *Tools) whoami(ctx context.Context, actor uuid.UUID, _ WhoamiInput) (IdentityOutput, error) {
	identity, err := t.service.Get(ctx, actor, actor)
	if err != nil {
		return IdentityOutput{}, translateError(err)
	}

	return toIdentityOutput(identity), nil
}
