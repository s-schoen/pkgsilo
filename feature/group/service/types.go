// Package service implements the group domain: named sets of identities and the rules for managing them.
package service

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/authz"
	"pkgsilo/core/request"
)

// NameRules are the rules of a group name, for the inputs of every transport that sets it: the request
// bodies of the HTTP API and the arguments of the MCP tools. It has 1 to 128 characters, not all
// whitespace, and no control characters. Callers must not modify them.
var NameRules = []validation.Rule{
	validation.Required,
	request.NotBlank,
	request.NoControlCharacters,
	validation.RuneLength(1, 128),
}

// Group is a named set of identities, its members, as an actor sees it: every field but the ID is hidden
// unless the actor may read it. The authz tag of a field names its action, such as group:read:name.
type Group struct {
	ID   uuid.UUID
	Name authz.Field[string] `authz:"name"`
	// CreatedBy and UpdatedBy are the identities, or audit.SystemActor, that made the changes.
	CreatedBy authz.Field[uuid.UUID] `authz:"createdBy"`
	UpdatedBy authz.Field[uuid.UUID] `authz:"updatedBy"`
	CreatedAt authz.Field[time.Time] `authz:"createdAt"`
	UpdatedAt authz.Field[time.Time] `authz:"updatedAt"`
}

// GroupInput holds the group fields set by callers when creating or replacing a group. The authz tag of a
// field names its actions, such as group:create:name and group:update:name.
type GroupInput struct {
	Name string `authz:"name"`
}

// GroupPatch holds the group fields that a caller changes when patching a group. A nil field stays as it is.
// Its members are tagged like those of GroupInput.
type GroupPatch struct {
	Name *string `authz:"name"`
}

// Member is the membership of an identity in a group.
type Member struct {
	IdentityID uuid.UUID
	// CreatedBy is the identity, or audit.SystemActor, that added the member, and CreatedAt is when.
	CreatedBy uuid.UUID
	CreatedAt time.Time
}
