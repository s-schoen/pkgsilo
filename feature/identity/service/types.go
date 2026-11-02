// Package service implements the identity domain: the users of pkgsilo and the rules for managing them.
package service

import (
	"time"
	"uuid"
)

// Identity is a user of pkgsilo.
type Identity struct {
	ID          uuid.UUID
	Username    string
	DisplayName string
	// CreatedBy and UpdatedBy are the identities, or audit.SystemActor, that made the changes.
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IdentityInput holds the identity fields set by callers when creating or replacing an identity.
type IdentityInput struct {
	Username    string
	DisplayName string
}
