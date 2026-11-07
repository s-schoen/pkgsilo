package service

import (
	"errors"

	"pkgsilo/feature/group/repository"
)

var (
	// ErrGroupNotFound is returned when no group has the requested ID, or the actor may not read it.
	ErrGroupNotFound = errors.New("group not found")
	// ErrGroupNameTaken is returned when another group already has the name.
	ErrGroupNameTaken = errors.New("group name already taken")
	// ErrIdentityNotFound is returned when the identity to add to a group doesn't exist, or the actor may not
	// read it.
	ErrIdentityNotFound = errors.New("identity not found")
	// ErrMemberNotFound is returned when the identity isn't a member of the group.
	ErrMemberNotFound = errors.New("identity is not a member of the group")
)

// translateError replaces repository errors with their domain errors.
func translateError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrGroupNotFound
	case errors.Is(err, repository.ErrDuplicateName):
		return ErrGroupNameTaken
	case errors.Is(err, repository.ErrIdentityNotFound):
		return ErrIdentityNotFound
	case errors.Is(err, repository.ErrMemberNotFound):
		return ErrMemberNotFound
	default:
		return err
	}
}
