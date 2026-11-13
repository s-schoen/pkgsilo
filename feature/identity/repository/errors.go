package repository

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNotFound is returned when no identity record has the requested ID.
	ErrNotFound = errors.New("identity record not found")
	// ErrDuplicateUsername is returned when another identity record already has the username.
	ErrDuplicateUsername = errors.New("duplicate identity username")
)

const (
	uniqueViolation       = "23505"
	usernameUniqueKeyName = "identities_username_key"
)

// translateError replaces PostgreSQL errors that have a repository meaning with their repository errors.
func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok &&
		pgErr.Code == uniqueViolation && pgErr.ConstraintName == usernameUniqueKeyName {
		return ErrDuplicateUsername
	}

	return err
}
