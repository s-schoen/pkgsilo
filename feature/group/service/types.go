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

// GroupPatch holds the group fields that a caller changes when patching a group. A nil field stays as it is.
type GroupPatch struct {
	Name *string
}

// fields returns the names of the fields that p sets.
func (p GroupPatch) fields() []string {
	var fields []string
	if p.Name != nil {
		fields = append(fields, "name")
	}

	return fields
}

// Member is the membership of an identity in a group.
type Member struct {
	IdentityID uuid.UUID
	// CreatedBy is the identity, or audit.SystemActor, that added the member, and CreatedAt is when.
	CreatedBy uuid.UUID
	CreatedAt time.Time
}
