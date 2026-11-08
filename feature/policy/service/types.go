// Package service implements the policy domain: the Cedar policies that decide what identities may do, and
// the checks that keep a policy with a mistake from being saved.
package service

import (
	"time"
	"uuid"

	"pkgsilo/core/authz"
)

// Policy is a named document of one or more Cedar statements, as an actor sees it: every field but the ID
// is hidden unless the actor may read it.
type Policy struct {
	ID     uuid.UUID
	Name   authz.Field[string]
	Source authz.Field[string]
	// CreatedBy and UpdatedBy are the identities, or audit.SystemActor, that made the changes.
	CreatedBy authz.Field[uuid.UUID]
	UpdatedBy authz.Field[uuid.UUID]
	CreatedAt authz.Field[time.Time]
	UpdatedAt authz.Field[time.Time]
}

// PolicyInput holds the policy fields set by callers when creating or replacing a policy.
type PolicyInput struct {
	Name   string
	Source string
}

// PolicyPatch holds the policy fields that a caller changes when patching a policy. A nil field stays as it
// is.
type PolicyPatch struct {
	Name   *string
	Source *string
}

// fields returns the names of the fields that p sets.
func (p PolicyPatch) fields() []string {
	var fields []string
	if p.Name != nil {
		fields = append(fields, "name")
	}
	if p.Source != nil {
		fields = append(fields, "source")
	}

	return fields
}
