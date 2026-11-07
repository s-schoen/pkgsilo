package service

import (
	"errors"

	"pkgsilo/feature/policy/repository"
)

var (
	// ErrPolicyNotFound is returned when no policy has the requested ID.
	ErrPolicyNotFound = errors.New("policy not found")
	// ErrPolicyNameTaken is returned when another policy already has the name.
	ErrPolicyNameTaken = errors.New("policy name already taken")
)

// InvalidPolicyError is returned when a policy can't be saved because of a mistake in its source, such as
// a typo in an action, or because it would take the policies over a limit.
type InvalidPolicyError struct {
	// Reason explains the mistake to the author of the policy.
	Reason string
}

func (e *InvalidPolicyError) Error() string {
	return "invalid policy: " + e.Reason
}

// translateError replaces repository errors with their domain errors.
func translateError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrPolicyNotFound
	case errors.Is(err, repository.ErrDuplicateName):
		return ErrPolicyNameTaken
	default:
		return err
	}
}
