package service

import (
	"errors"

	"pkgsilo/feature/identity/repository"
)

var (
	// ErrIdentityNotFound is returned when no identity has the requested ID.
	ErrIdentityNotFound = errors.New("identity not found")
	// ErrUsernameTaken is returned when another identity already has the username.
	ErrUsernameTaken = errors.New("username already taken")
)

// translateError replaces repository errors with their domain errors.
func translateError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrIdentityNotFound
	case errors.Is(err, repository.ErrDuplicateUsername):
		return ErrUsernameTaken
	default:
		return err
	}
}
