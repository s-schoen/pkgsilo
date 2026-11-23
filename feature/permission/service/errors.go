package service

import (
	"errors"

	"pkgsilo/core/authz"
)

// ErrIdentityNotFound is returned when the identity whose permissions are asked for, or the owner they're
// asked for, doesn't exist, or the actor may not read it.
var ErrIdentityNotFound = errors.New("identity not found")

// translateError replaces the errors of the owner loader with their domain errors.
func translateError(err error) error {
	if errors.Is(err, authz.ErrOwnerNotFound) {
		return ErrIdentityNotFound
	}

	return err
}
