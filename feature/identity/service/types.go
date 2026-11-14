// Package service implements the identity domain: the users of pkgsilo and the rules for managing them.
package service

import (
	"regexp"
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/authz"
	"pkgsilo/core/request"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)

// The rules of the identity fields that callers set, for the inputs of every transport that sets them: the
// request bodies of the HTTP API and the arguments of the MCP tools. Callers must not modify them.
var (
	// UsernameRules are the rules of a username: 3 to 64 lowercase letters and digits, optionally separated by
	// single '.', '_' or '-'.
	UsernameRules = []validation.Rule{
		validation.Required,
		validation.Length(3, 64),
		validation.Match(usernamePattern).
			Error("must consist of lowercase letters and digits, optionally separated by single '.', '_' or '-'"),
	}
	// DisplayNameRules are the rules of a display name: 1 to 128 characters, not all whitespace, and no
	// control characters.
	DisplayNameRules = []validation.Rule{
		validation.Required,
		request.NotBlank,
		request.NoControlCharacters,
		validation.RuneLength(1, 128),
	}
)

// Identity is a user of pkgsilo, as an actor sees it: every field but the ID is hidden unless the actor may
// read it. The authz tag of a field names its action, such as identity:read:username.
type Identity struct {
	ID          uuid.UUID
	Username    authz.Field[string] `authz:"username"`
	DisplayName authz.Field[string] `authz:"displayName"`
	// CreatedBy and UpdatedBy are the identities, or audit.SystemActor, that made the changes.
	CreatedBy authz.Field[uuid.UUID] `authz:"createdBy"`
	UpdatedBy authz.Field[uuid.UUID] `authz:"updatedBy"`
	CreatedAt authz.Field[time.Time] `authz:"createdAt"`
	UpdatedAt authz.Field[time.Time] `authz:"updatedAt"`
}

// IdentityInput holds the identity fields set by callers when creating or replacing an identity. The authz
// tag of a field names its actions, such as identity:create:username and identity:update:username.
type IdentityInput struct {
	Username    string `authz:"username"`
	DisplayName string `authz:"displayName"`
}

// IdentityPatch holds the identity fields that a caller changes when patching an identity. A nil field
// stays as it is. Its members are tagged like those of IdentityInput.
type IdentityPatch struct {
	Username    *string `authz:"username"`
	DisplayName *string `authz:"displayName"`
}
