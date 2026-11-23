package service

import (
	"uuid"

	"pkgsilo/core/authz"
)

// Permissions are the actions that the policies may allow an identity, the actor itself, to do.
type Permissions struct {
	// ID is the identity's ID.
	ID uuid.UUID
	// AllowedActions holds the names of the allowed actions, in the order of the types and of their actions.
	AllowedActions authz.AllowedActions
}
