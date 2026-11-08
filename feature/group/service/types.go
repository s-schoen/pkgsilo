// Package service implements the group domain: named sets of identities and the rules for managing them.
package service

import (
	"time"
	"uuid"

	"pkgsilo/core/authz"
)

// Group is a named set of identities, its members, as an actor sees it: every field but the ID is hidden
// unless the actor may read it.
type Group struct {
	ID   uuid.UUID
	Name authz.Field[string]
	// CreatedBy and UpdatedBy are the identities, or audit.SystemActor, that made the changes.
	CreatedBy authz.Field[uuid.UUID]
	UpdatedBy authz.Field[uuid.UUID]
	CreatedAt authz.Field[time.Time]
	UpdatedAt authz.Field[time.Time]
}

// GroupInput holds the group fields set by callers when creating or replacing a group.
type GroupInput struct {
	Name string
}

// Member is the membership of an identity in a group.
type Member struct {
	IdentityID uuid.UUID
	// CreatedBy is the identity, or audit.SystemActor, that added the member, and CreatedAt is when.
	CreatedBy uuid.UUID
	CreatedAt time.Time
}
