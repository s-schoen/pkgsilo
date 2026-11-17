package repository

import (
	"database/sql"
	"errors"
)

var (
	// ErrNotFound is returned when no session record matches the requested ID or hash.
	ErrNotFound = errors.New("session record not found")
	// ErrPasswordChanged is returned when the password of a record's identity no longer has the hash that
	// its login verified, since it was changed or deleted after the login read it, or the identity was
	// deleted.
	ErrPasswordChanged = errors.New("session password changed")
)

// translateError replaces PostgreSQL errors that have a repository meaning with their repository errors.
func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	return err
}
