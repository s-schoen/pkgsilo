package repository

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNotFound is returned when no group record has the requested ID.
	ErrNotFound = errors.New("group record not found")
	// ErrDuplicateName is returned when another group record already has the name.
	ErrDuplicateName = errors.New("duplicate group name")
	// ErrIdentityNotFound is returned when a member record refers to an identity that doesn't exist.
	ErrIdentityNotFound = errors.New("member identity not found")
	// ErrMemberNotFound is returned when the group has no member record for the identity.
	ErrMemberNotFound = errors.New("member record not found")
)

const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"

	nameUniqueKeyName            = "groups_name_key"
	memberGroupForeignKeyName    = "group_members_group_id_fkey"
	memberIdentityForeignKeyName = "group_members_identity_id_fkey"
)

// translateError replaces PostgreSQL errors that have a repository meaning with their repository errors.
func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch {
	case pgErr.Code == uniqueViolation && pgErr.ConstraintName == nameUniqueKeyName:
		return ErrDuplicateName
	case pgErr.Code == foreignKeyViolation && pgErr.ConstraintName == memberGroupForeignKeyName:
		return ErrNotFound
	case pgErr.Code == foreignKeyViolation && pgErr.ConstraintName == memberIdentityForeignKeyName:
		return ErrIdentityNotFound
	default:
		return err
	}
}
