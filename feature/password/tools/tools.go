package tools

import (
	"context"
	"uuid"

	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/password/service"
)

// Tools offers the password tools.
type Tools struct {
	service service.PasswordService
}

var _ mcpserver.Toolset = (*Tools)(nil)

// New returns the password tools, which read and delete passwords through svc.
func New(svc service.PasswordService) *Tools {
	return &Tools{service: svc}
}

// Parts of the tools' descriptions that several tools share.
const (
	// passwordTerm says what a password is, and why no tool sets one.
	passwordTerm = "A password lets an identity, a user of pkgsilo, log in with its username, besides its API " +
		"keys. An identity has at most one password. No tool sets a password or returns one: a plaintext " +
		"password must not pass through a model's context or logs, so passwords are set through the REST API " +
		"only. "
	// masking says what the output holds.
	masking = "Fields that the policies do not allow you to read are absent from the output. "
	// identityNotFound says what identity_not_found means.
	identityNotFound = "Fails with identity_not_found if the identity does not exist, or if the policies do not " +
		"allow you to read it: the two cases look the same. "
	// notFound says what password_not_found means.
	notFound = "Fails with password_not_found if the identity has no password, or if the policies do not allow " +
		"you to read it: the cases look the same. "
)

// Register adds the password tools to reg.
func (t *Tools) Register(reg *mcpserver.Registry) {
	reg.Add(mcpserver.Tool{
		Name: "password_get",
		Description: "Returns when the password of an identity was last set. " + passwordTerm +
			"Use it to find out whether an identity can log in with a password, and how old its password is. " +
			masking + identityNotFound + notFound,
		Kind: mcpserver.Read,
	}, t.get)
	reg.Add(mcpserver.Tool{
		Name: "password_delete",
		Description: "Deletes the password of an identity, which can't log in with it from then on and can only " +
			"use its API keys. It can't be undone; a new password can only be set through the REST API. " +
			passwordTerm + "The output is empty. " + identityNotFound + notFound +
			"Fails with forbidden if the policies allow you to read the password, but not to delete it; nothing " +
			"is changed then.",
		Kind: mcpserver.Delete,
	}, t.delete)
}

func (t *Tools) get(ctx context.Context, actor uuid.UUID, in PasswordInput) (PasswordOutput, error) {
	password, err := t.service.Get(ctx, actor, in.IdentityID)
	if err != nil {
		return PasswordOutput{}, translateError(err)
	}

	return toPasswordOutput(password), nil
}

func (t *Tools) delete(ctx context.Context, actor uuid.UUID, in PasswordInput) (mcpserver.DoneOutput, error) {
	if err := t.service.Delete(ctx, actor, in.IdentityID); err != nil {
		return mcpserver.DoneOutput{}, translateError(err)
	}

	return mcpserver.DoneOutput{}, nil
}
