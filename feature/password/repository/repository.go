// Package repository persists the passwords of identities.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"uuid"

	"pkgsilo/core/database"
)

// PasswordRepository stores password records.
type PasswordRepository interface {
	// Get returns the record of the identity's password.
	Get(ctx context.Context, identityID uuid.UUID) (PasswordRecord, error)
	// Set stores the record as the identity's password, replacing the one it has, and returns it as stored.
	// It returns ErrIdentityNotFound if the identity doesn't exist.
	Set(ctx context.Context, rec PasswordRecord) (PasswordRecord, error)
	// Change replaces the identity's password with the record, but only while its hash is still oldHash, so
	// that a change checked against the old hash can't overwrite one that happened since. It returns the
	// record as stored, or ErrHashChanged if the password was changed or deleted since oldHash was read.
	Change(ctx context.Context, rec PasswordRecord, oldHash string) (PasswordRecord, error)
	// UpdateHash replaces the hash of the identity's password with newHash, but only while it's still
	// oldHash, so that a rehash can't undo a change that happened since the old hash was read. It reports
	// whether it replaced the hash.
	UpdateHash(ctx context.Context, identityID uuid.UUID, oldHash, newHash string) (bool, error)
	// Delete removes the identity's password.
	Delete(ctx context.Context, identityID uuid.UUID) error
	// GetCredentials returns the identity with the given username and the hash of its password, if it has
	// one. It returns ErrIdentityNotFound if no identity has the username.
	GetCredentials(ctx context.Context, username string) (CredentialsRecord, error)
}

const passwordColumns = "identity_id, hash, set_at"

// PostgresPasswordRepository stores password records in the passwords table of PostgreSQL.
type PostgresPasswordRepository struct {
	db database.Querier
}

var _ PasswordRepository = (*PostgresPasswordRepository)(nil)

// NewPostgresPasswordRepository returns a PostgresPasswordRepository that stores records in db.
func NewPostgresPasswordRepository(db database.Querier) *PostgresPasswordRepository {
	return &PostgresPasswordRepository{db: db}
}

// Get returns the record of the identity's password.
func (r *PostgresPasswordRepository) Get(ctx context.Context, identityID uuid.UUID) (PasswordRecord, error) {
	var rec PasswordRecord
	err := r.db.GetContext(ctx, &rec, `SELECT `+passwordColumns+` FROM passwords WHERE identity_id = $1`, identityID)
	if err != nil {
		return PasswordRecord{}, fmt.Errorf("get password: %w", translateError(err))
	}

	return rec, nil
}

// Set stores the record as the identity's password, replacing the one it has, and returns it as stored.
func (r *PostgresPasswordRepository) Set(ctx context.Context, rec PasswordRecord) (PasswordRecord, error) {
	var stored PasswordRecord
	err := r.db.GetContext(ctx, &stored, `
		INSERT INTO passwords (`+passwordColumns+`)
		VALUES ($1, $2, $3)
		ON CONFLICT (identity_id) DO UPDATE SET hash = excluded.hash, set_at = excluded.set_at
		RETURNING `+passwordColumns,
		rec.IdentityID, rec.Hash, rec.SetAt,
	)
	if err != nil {
		return PasswordRecord{}, fmt.Errorf("set password: %w", translateError(err))
	}

	return stored, nil
}

// Change replaces the identity's password with the record, but only while its hash is still oldHash, in one
// statement, which waits for a concurrent write of the password and then checks the hash it left.
func (r *PostgresPasswordRepository) Change(
	ctx context.Context, rec PasswordRecord, oldHash string,
) (PasswordRecord, error) {
	var stored PasswordRecord
	err := r.db.GetContext(ctx, &stored, `
		UPDATE passwords SET hash = $3, set_at = $4
		WHERE identity_id = $1 AND hash = $2
		RETURNING `+passwordColumns,
		rec.IdentityID, oldHash, rec.Hash, rec.SetAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PasswordRecord{}, fmt.Errorf("change password: %w", ErrHashChanged)
	}
	if err != nil {
		return PasswordRecord{}, fmt.Errorf("change password: %w", err)
	}

	return stored, nil
}

// UpdateHash replaces the hash of the identity's password with newHash, but only while it's still oldHash.
func (r *PostgresPasswordRepository) UpdateHash(
	ctx context.Context, identityID uuid.UUID, oldHash, newHash string,
) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE passwords SET hash = $3 WHERE identity_id = $1 AND hash = $2`, identityID, oldHash, newHash)
	if err != nil {
		return false, fmt.Errorf("update password hash: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("update password hash: %w", err)
	}

	return n > 0, nil
}

// Delete removes the identity's password.
func (r *PostgresPasswordRepository) Delete(ctx context.Context, identityID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM passwords WHERE identity_id = $1`, identityID)
	if err != nil {
		return fmt.Errorf("delete password: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete password: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("delete password: %w", ErrNotFound)
	}

	return nil
}

// GetCredentials returns the identity with the given username and the hash of its password, if it has one.
// One query tells both, so that the password can't change between them.
func (r *PostgresPasswordRepository) GetCredentials(ctx context.Context, username string) (CredentialsRecord, error) {
	var rec CredentialsRecord
	err := r.db.GetContext(ctx, &rec, `
		SELECT i.id AS identity_id, p.hash FROM identities i
		LEFT JOIN passwords p ON p.identity_id = i.id
		WHERE i.username = $1`,
		username,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CredentialsRecord{}, fmt.Errorf("get credentials: %w", ErrIdentityNotFound)
	}
	if err != nil {
		return CredentialsRecord{}, fmt.Errorf("get credentials: %w", err)
	}

	return rec, nil
}
