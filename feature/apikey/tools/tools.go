package tools

import (
	"context"
	"uuid"

	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/apikey/service"
)

// Tools offers the API key tools.
type Tools struct {
	service service.APIKeyService
}

var _ mcpserver.Toolset = (*Tools)(nil)

// New returns the API key tools, which read and revoke API keys through svc.
func New(svc service.APIKeyService) *Tools {
	return &Tools{service: svc}
}

// Parts of the tools' descriptions that several tools share.
const (
	// apiKeyTerm says what an API key is, and that the tools never show or create keys.
	apiKeyTerm = "An API key is a credential of an identity, a user of pkgsilo: each request that presents its " +
		"key acts as the identity, with what the policies give the identity. An identity can have several " +
		"API keys, such as one per machine, each with a name. The key itself is shown only once, when the API " +
		"key is created, which is done through the REST API only; no tool returns or creates a key, and " +
		"keySuffix, the key's last characters, tells which API key a key belongs to. "
	// masking says what the output holds.
	masking = "Fields that the policies do not allow you to read are absent from the output. createdBy is the " +
		"nil UUID for an API key that pkgsilo created itself, such as the bootstrap key of the first start. "
	// identityNotFound says what identity_not_found means.
	identityNotFound = "Fails with identity_not_found if the identity does not exist, or if the policies do not " +
		"allow you to read it: the two cases look the same. "
	// notFound says what apikey_not_found means.
	notFound = "Fails with apikey_not_found if the identity has no API key with the ID, such as one of " +
		"another identity, or if the policies do not allow you to read it: the cases look the same. "
	// paging says how to page through a list.
	paging = "Page through the list by calling it again with offset set to the nextOffset of the previous " +
		"page, until there is no nextOffset. "
)

// Register adds the API key tools to reg.
func (t *Tools) Register(reg *mcpserver.Registry) {
	reg.Add(mcpserver.Tool{
		Name: "apikey_list",
		Description: "Lists the API keys of an identity that the policies allow you to read, oldest first, one " +
			"page at a time. " + apiKeyTerm +
			"Use it to review which API keys an identity has and when they were last used, or to find the API " +
			"key to revoke with apikey_delete, such as by its name or keySuffix. " + paging +
			"API keys that you may not read are left out, as if they did not exist, though they still work. " +
			masking + identityNotFound,
		Kind: mcpserver.Read,
	}, t.list)
	reg.Add(mcpserver.Tool{
		Name: "apikey_get",
		Description: "Returns the API key with the given ID of an identity. " + apiKeyTerm + masking +
			identityNotFound + notFound,
		Kind: mcpserver.Read,
	}, t.get)
	reg.Add(mcpserver.Tool{
		Name: "apikey_delete",
		Description: "Deletes an API key of an identity, which revokes it at once: every request that presents " +
			"its key is rejected from then on, while the identity's other API keys keep working. It can't be " +
			"undone; a new API key can only be created through the REST API. " + apiKeyTerm +
			"Deleting the API key that you authenticate with ends your own access. The output is empty. " +
			identityNotFound + notFound +
			"Fails with forbidden if the policies allow you to read the API key, but not to delete it; nothing " +
			"is changed then.",
		Kind: mcpserver.Delete,
	}, t.delete)
}

func (t *Tools) list(
	ctx context.Context, actor uuid.UUID, in ListInput,
) (mcpserver.ListOutput[APIKeyOutput], error) {
	page := in.Pagination()
	keys, err := t.service.List(ctx, actor, in.IdentityID, page.Offset, page.Limit)
	if err != nil {
		return mcpserver.ListOutput[APIKeyOutput]{}, translateError(err)
	}

	return mcpserver.NewListOutput(keys, page.Offset, toAPIKeyOutput), nil
}

func (t *Tools) get(ctx context.Context, actor uuid.UUID, in KeyInput) (APIKeyOutput, error) {
	key, err := t.service.Get(ctx, actor, in.IdentityID, in.ID)
	if err != nil {
		return APIKeyOutput{}, translateError(err)
	}

	return toAPIKeyOutput(key), nil
}

func (t *Tools) delete(ctx context.Context, actor uuid.UUID, in KeyInput) (mcpserver.DoneOutput, error) {
	if err := t.service.Delete(ctx, actor, in.IdentityID, in.ID); err != nil {
		return mcpserver.DoneOutput{}, translateError(err)
	}

	return mcpserver.DoneOutput{}, nil
}
