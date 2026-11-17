package service

import (
	"errors"
	"uuid"

	"pkgsilo/feature/session/repository"
)

var (
	// ErrSessionNotFound is returned when the actor has no session with the requested ID.
	ErrSessionNotFound = errors.New("session not found")
	// ErrMalformedToken is returned when a presented token doesn't have the format of a token.
	ErrMalformedToken = errors.New("malformed token")
	// ErrUnknownToken is returned when a presented token belongs to no session.
	ErrUnknownToken = errors.New("unknown token")
	// ErrExpiredSession is returned when a presented token belongs to a session that has expired.
	ErrExpiredSession = errors.New("expired session")
	// ErrUnknownCredentials is the reason of a rejected login when no identity has the username, or the
	// identity has no password.
	ErrUnknownCredentials = errors.New("unknown credentials")
	// ErrInvalidPassword is the reason of a rejected login when the password doesn't match the identity's.
	ErrInvalidPassword = errors.New("invalid password")
)

// RejectedError reports that a login's username and password aren't accepted. It matches its Reason.
type RejectedError struct {
	// IdentityID is the identity with the username, or the nil UUID if no identity has it. It's for the
	// logs only: no session is created for it.
	IdentityID uuid.UUID
	// Reason is ErrUnknownCredentials or ErrInvalidPassword.
	Reason error
}

func (e *RejectedError) Error() string {
	return "login rejected: " + e.Reason.Error()
}

// Unwrap returns the reason.
func (e *RejectedError) Unwrap() error {
	return e.Reason
}

// translateError replaces repository errors with their domain errors.
func translateError(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrSessionNotFound
	}

	return err
}
