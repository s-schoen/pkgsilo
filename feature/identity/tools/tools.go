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

// Parts of the tools' descriptions that several tools share.
const (
	// identityTerm says what an identity is, for the tools that don't start from the actor's own.
	identityTerm = "An identity is a user of pkgsilo, a person or an agent, with a unique username and a " +
		"display name. It authenticates with API keys, and the policies of pkgsilo decide what it may do. "
	// masking says what the output holds.
	masking = "Fields that the policies do not allow you to read are absent from the output. createdBy and " +
		"updatedBy are the nil UUID for changes that pkgsilo made itself, such as creating the first identity. "
	// notFound says what identity_not_found means.
	notFound = "Fails with identity_not_found if no identity has the ID, or if the policies do not allow you " +
		"to read it: the two cases look the same. "
	// fieldsDenied says what fields_denied means for a tool that writes.
	fieldsDenied = "Fails with fields_denied, naming the arguments, if the policies do not allow you to set " +
		"some of them, and with invalid_arguments, naming each invalid argument and why, if they break the " +
		"rules; nothing is changed then."
)

// Register adds the identity tools to reg.
func (t *Tools) Register(reg *mcpserver.Registry) {
	reg.Add(mcpserver.Tool{
		Name: "identity_list",
		Description: "Lists the identities that the policies allow you to read, oldest first, one page at a " +
			"time. " + identityTerm +
			"Use it to find an identity, such as by its username, since there is no search: page through the " +
			"list by calling it again with offset set to the nextOffset of the previous page, until there is " +
			"no nextOffset. Identities that you may not read are left out, as if they did not exist. " + masking,
		Kind: mcpserver.Read,
	}, t.list)
	reg.Add(mcpserver.Tool{
		Name:        "identity_get",
		Description: "Returns the identity with the given ID. " + identityTerm + masking + notFound,
		Kind:        mcpserver.Read,
	}, t.get)
	reg.Add(mcpserver.Tool{
		Name: "identity_create",
		Description: "Creates an identity, a new user of pkgsilo, and returns it. " + identityTerm +
			"The new identity has no API key and belongs to no group, so it can't authenticate and only the " +
			"policies for every identity apply to it. The output holds only its id if the policies do not " +
			"allow you to read it. " + masking +
			"Fails with username_taken if another identity has the username. " + fieldsDenied,
		Kind: mcpserver.Write,
	}, t.create)
	reg.Add(mcpserver.Tool{
		Name: "identity_update",
		Description: "Changes the username or the display name of an identity, or both, and returns the " +
			"identity. " + identityTerm +
			"Set only the arguments to change: those that you leave out keep their values, even if they " +
			"change at the same time. With neither set, it changes nothing. " + masking + notFound +
			"Fails with username_taken if another identity has the new username. " + fieldsDenied,
		Kind: mcpserver.Write,
	}, t.update)
	reg.Add(mcpserver.Tool{
		Name: "identity_whoami",
		Description: "Returns the identity that you act as: the user of pkgsilo whose API key authenticates you. " +
			"Every other tool acts as this identity, and the policies of pkgsilo decide what it may do. " +
			masking + "Fails with identity_not_found if the policies do not allow you to read your own identity.",
		Kind: mcpserver.Read,
	}, t.whoami)
}

func (t *Tools) list(
	ctx context.Context, actor uuid.UUID, in mcpserver.ListInput,
) (mcpserver.ListOutput[IdentityOutput], error) {
	page := in.Pagination()
	identities, err := t.service.List(ctx, actor, page.Offset, page.Limit)
	if err != nil {
		return mcpserver.ListOutput[IdentityOutput]{}, translateError(err)
	}

	return mcpserver.NewListOutput(identities, page.Offset, toIdentityOutput), nil
}

func (t *Tools) get(ctx context.Context, actor uuid.UUID, in GetInput) (IdentityOutput, error) {
	identity, err := t.service.Get(ctx, actor, in.ID)
	if err != nil {
		return IdentityOutput{}, translateError(err)
	}

	return toIdentityOutput(identity), nil
}

func (t *Tools) create(ctx context.Context, actor uuid.UUID, in CreateInput) (IdentityOutput, error) {
	identity, err := t.service.Create(ctx, actor, in.toInput())
	if err != nil {
		return IdentityOutput{}, translateError(err)
	}

	return toIdentityOutput(identity), nil
}

func (t *Tools) update(ctx context.Context, actor uuid.UUID, in UpdateInput) (IdentityOutput, error) {
	identity, err := t.service.Patch(ctx, actor, in.ID, in.toPatch())
	if err != nil {
		return IdentityOutput{}, translateError(err)
	}

	return toIdentityOutput(identity), nil
}

func (t *Tools) whoami(ctx context.Context, actor uuid.UUID, _ WhoamiInput) (IdentityOutput, error) {
	identity, err := t.service.Get(ctx, actor, actor)
	if err != nil {
		return IdentityOutput{}, translateError(err)
	}

	return toIdentityOutput(identity), nil
}
