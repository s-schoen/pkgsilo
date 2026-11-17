package service

import (
	"errors"
	"uuid"

	"pkgsilo/core/authz"
	"pkgsilo/feature/password/repository"
)

var (
	// ErrPasswordNotFound is returned when the identity has no password, or the actor may not read it.
	ErrPasswordNotFound = errors.New("password not found")
	// ErrIdentityNotFound is returned when the identity of a password doesn't exist, or the actor may not
	// read it.
	ErrIdentityNotFound = errors.New("identity not found")
	// ErrCurrentPasswordRequired is returned when an actor that may change a password but not reset it
	// leaves out the current password.
	ErrCurrentPasswordRequired = errors.New("current password required")
	// ErrWrongCurrentPassword is returned when the current password that an actor gives to change a password
	// doesn't match it, which includes a password that's changed or deleted while the change is checked.
	ErrWrongCurrentPassword = errors.New("wrong current password")
	// ErrUnknownCredentials is the reason of a rejection when no identity has the username, or the identity
	// has no password.
	ErrUnknownCredentials = errors.New("unknown credentials")
	// ErrInvalidPassword is the reason of a rejection when the password doesn't match the identity's.
	ErrInvalidPassword = errors.New("invalid password")
)

// RejectedError reports that a username and password aren't accepted. It matches its Reason.
type RejectedError struct {
	// IdentityID is the identity with the username, or the nil UUID if no identity has it. It's for the
	// logs only: the request isn't authenticated as it.
	IdentityID uuid.UUID
	// Reason is ErrUnknownCredentials or ErrInvalidPassword.
	Reason error
}

func (e *RejectedError) Error() string {
	return "password rejected: " + e.Reason.Error()
}

// Unwrap returns the reason.
func (e *RejectedError) Unwrap() error {
	return e.Reason
}

// translateError replaces the errors of the repository and of the owner loader with their domain errors.
func translateError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrPasswordNotFound
	case errors.Is(err, repository.ErrIdentityNotFound), errors.Is(err, authz.ErrOwnerNotFound):
		return ErrIdentityNotFound
	case errors.Is(err, repository.ErrHashChanged):
		// The current password was checked against a hash that's gone: it's no longer the current one.
		return ErrWrongCurrentPassword
	default:
		return err
	}
}
