// Package service implements the identity domain: the users of pkgsilo and the rules for managing them.
package service

import (
	"time"
	"uuid"

	"pkgsilo/core/authz"
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
