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
	// ErrTooManyRows is returned when loading all records would exceed pagination.MaxAllRows.
	ErrTooManyRows = errors.New("too many identity records to load")
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

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == usernameUniqueKeyName {
		return ErrDuplicateUsername
	}

	return err
}
