// Package service implements the policy domain: the Cedar policies that decide what identities may do, and
// the checks that keep a policy with a mistake from being saved.
package service

import (
	"time"
	"uuid"

	"pkgsilo/core/authz"
)

// Policy is a named document of one or more Cedar statements, as an actor sees it: every field but the ID
// is hidden unless the actor may read it. The authz tag of a field names its action, such as
// policy:read:source.
type Policy struct {
	ID     uuid.UUID
	Name   authz.Field[string] `authz:"name"`
	Source authz.Field[string] `authz:"source"`
	// CreatedBy and UpdatedBy are the identities, or audit.SystemActor, that made the changes.
	CreatedBy authz.Field[uuid.UUID] `authz:"createdBy"`
	UpdatedBy authz.Field[uuid.UUID] `authz:"updatedBy"`
	CreatedAt authz.Field[time.Time] `authz:"createdAt"`
	UpdatedAt authz.Field[time.Time] `authz:"updatedAt"`
}

// PolicyInput holds the policy fields set by callers when creating or replacing a policy. The authz tag of a
// field names its actions, such as policy:create:source and policy:update:source.
type PolicyInput struct {
	Name   string `authz:"name"`
	Source string `authz:"source"`
}

// PolicyPatch holds the policy fields that a caller changes when patching a policy. A nil field stays as it
// is. Its members are tagged like those of PolicyInput.
type PolicyPatch struct {
	Name   *string `authz:"name"`
	Source *string `authz:"source"`
}
