// Package service implements the group domain: named sets of identities and the rules for managing them.
package service

import (
	"time"
	"uuid"
)

// SystemActor attributes changes to pkgsilo itself rather than to an identity.
var SystemActor = uuid.Nil()

// Group is a named set of identities, its members.
type Group struct {
	ID   uuid.UUID
	Name string
	// CreatedBy and UpdatedBy are the identities, or SystemActor, that made the changes.
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GroupInput holds the group fields set by callers when creating or replacing a group.
type GroupInput struct {
	Name string
}

// Member is the membership of an identity in a group.
type Member struct {
	IdentityID uuid.UUID
	// CreatedBy is the identity, or SystemActor, that added the member, and CreatedAt is when.
	CreatedBy uuid.UUID
	CreatedAt time.Time
}
