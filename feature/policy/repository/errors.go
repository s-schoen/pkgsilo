package repository

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNotFound is returned when no policy record has the requested ID.
	ErrNotFound = errors.New("policy record not found")
	// ErrDuplicateName is returned when another policy record already has the name.
	ErrDuplicateName = errors.New("duplicate policy name")
)

const (
	uniqueViolation   = "23505"
	nameUniqueKeyName = "policies_name_key"
)

// translateError replaces PostgreSQL errors that have a repository meaning with their repository errors.
func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == nameUniqueKeyName {
		return ErrDuplicateName
	}

	return err
}
