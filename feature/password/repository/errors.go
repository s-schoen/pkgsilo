package repository

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNotFound is returned when the identity has no password.
	ErrNotFound = errors.New("password record not found")
	// ErrIdentityNotFound is returned when the identity doesn't exist.
	ErrIdentityNotFound = errors.New("password identity not found")
)

const (
	foreignKeyViolation    = "23503"
	identityForeignKeyName = "passwords_identity_id_fkey"
)

// translateError replaces PostgreSQL errors that have a repository meaning with their repository errors.
func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok &&
		pgErr.Code == foreignKeyViolation && pgErr.ConstraintName == identityForeignKeyName {
		return ErrIdentityNotFound
	}

	return err
}
