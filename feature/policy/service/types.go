// Package service implements the policy domain: the Cedar policies that decide what identities may do, and
// the checks that keep a policy with a mistake from being saved.
package service

import (
	"time"
	"uuid"
)

// Policy is a named document of one or more Cedar statements.
type Policy struct {
	ID     uuid.UUID
	Name   string
	Source string
	// CreatedBy and UpdatedBy are the identities, or audit.SystemActor, that made the changes.
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PolicyInput holds the policy fields set by callers when creating or replacing a policy.
type PolicyInput struct {
	Name   string
	Source string
}
