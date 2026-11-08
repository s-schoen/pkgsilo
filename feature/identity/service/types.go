// Package service implements the identity domain: the users of pkgsilo and the rules for managing them.
package service

import (
	"time"
	"uuid"

	"pkgsilo/core/authz"
)

// Identity is a user of pkgsilo, as an actor sees it: every field but the ID is hidden unless the actor may
// read it.
type Identity struct {
	ID          uuid.UUID
	Username    authz.Field[string]
	DisplayName authz.Field[string]
	// CreatedBy and UpdatedBy are the identities, or audit.SystemActor, that made the changes.
	CreatedBy authz.Field[uuid.UUID]
	UpdatedBy authz.Field[uuid.UUID]
	CreatedAt authz.Field[time.Time]
	UpdatedAt authz.Field[time.Time]
}

// IdentityInput holds the identity fields set by callers when creating or replacing an identity.
type IdentityInput struct {
	Username    string
	DisplayName string
}

// IdentityPatch holds the identity fields that a caller changes when patching an identity. A nil field
// stays as it is.
type IdentityPatch struct {
	Username    *string
	DisplayName *string
}

// fields returns the names of the fields that p sets.
func (p IdentityPatch) fields() []string {
	var fields []string
	if p.Username != nil {
		fields = append(fields, "username")
	}
	if p.DisplayName != nil {
		fields = append(fields, "displayName")
	}

	return fields
}
