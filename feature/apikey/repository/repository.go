// Package repository persists API keys.
package repository

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"pkgsilo/core/database"
)

// APIKeyRepository stores API key records.
type APIKeyRepository interface {
	// Insert stores a new record and returns it as stored.
	Insert(ctx context.Context, rec APIKeyRecord) (APIKeyRecord, error)
	// Get returns the record with the given ID of the given identity.
	Get(ctx context.Context, identityID, id uuid.UUID) (APIKeyRecord, error)
	// GetByHash returns the record with the given key hash, whatever its identity.
	GetByHash(ctx context.Context, keyHash []byte) (APIKeyRecord, error)
	// List returns up to limit records of the given identity, skipping the first offset, ordered by
	// creation time.
	List(ctx context.Context, identityID uuid.UUID, offset, limit int) ([]APIKeyRecord, error)
	// UpdateLastUsed sets the last use of the record with the given ID. A record that no longer
	// exists is not an error: its key was deleted while it was in use.
	UpdateLastUsed(ctx context.Context, id uuid.UUID, lastUsedAt time.Time) error
	// Delete removes the record with the given ID of the given identity.
	Delete(ctx context.Context, identityID, id uuid.UUID) error
	// IdentityExists reports whether the identity with the given ID exists.
	IdentityExists(ctx context.Context, identityID uuid.UUID) (bool, error)
}

const keyColumns = "id, identity_id, name, key_hash, key_suffix, expires_at, last_used_at, created_by, created_at"

// PostgresAPIKeyRepository stores API key records in the api_keys table of PostgreSQL.
type PostgresAPIKeyRepository struct {
	db database.Querier
}

var _ APIKeyRepository = (*PostgresAPIKeyRepository)(nil)

// NewPostgresAPIKeyRepository returns a PostgresAPIKeyRepository that stores records in db.
func NewPostgresAPIKeyRepository(db database.Querier) *PostgresAPIKeyRepository {
	return &PostgresAPIKeyRepository{db: db}
}

// Insert stores a new record and returns it as stored.
func (r *PostgresAPIKeyRepository) Insert(ctx context.Context, rec APIKeyRecord) (APIKeyRecord, error) {
	var stored APIKeyRecord
	err := r.db.GetContext(ctx, &stored, `
		INSERT INTO api_keys (`+keyColumns+`)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+keyColumns,
		rec.ID, rec.IdentityID, rec.Name, rec.KeyHash, rec.KeySuffix, rec.ExpiresAt, rec.LastUsedAt, rec.CreatedBy,
		rec.CreatedAt,
	)
	if err != nil {
		return APIKeyRecord{}, fmt.Errorf("insert api key: %w", translateError(err))
	}

	return stored, nil
}

// Get returns the record with the given ID of the given identity.
func (r *PostgresAPIKeyRepository) Get(ctx context.Context, identityID, id uuid.UUID) (APIKeyRecord, error) {
	var rec APIKeyRecord
	err := r.db.GetContext(ctx, &rec,
		`SELECT `+keyColumns+` FROM api_keys WHERE identity_id = $1 AND id = $2`, identityID, id)
	if err != nil {
		return APIKeyRecord{}, fmt.Errorf("get api key: %w", translateError(err))
	}

	return rec, nil
}

// GetByHash returns the record with the given key hash, whatever its identity.
func (r *PostgresAPIKeyRepository) GetByHash(ctx context.Context, keyHash []byte) (APIKeyRecord, error) {
	var rec APIKeyRecord
	err := r.db.GetContext(ctx, &rec, `SELECT `+keyColumns+` FROM api_keys WHERE key_hash = $1`, keyHash)
	if err != nil {
		return APIKeyRecord{}, fmt.Errorf("get api key by hash: %w", translateError(err))
	}

	return rec, nil
}

// List returns up to limit records of the given identity, skipping the first offset, ordered by
// creation time.
func (r *PostgresAPIKeyRepository) List(
	ctx context.Context, identityID uuid.UUID, offset, limit int,
) ([]APIKeyRecord, error) {
	var recs []APIKeyRecord
	err := r.db.SelectContext(ctx, &recs, `
		SELECT `+keyColumns+` FROM api_keys
		WHERE identity_id = $1
		ORDER BY created_at, id
		LIMIT $2 OFFSET $3`,
		identityID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}

	return recs, nil
}

// UpdateLastUsed sets the last use of the record with the given ID. A record that no longer exists
// is not an error: its key was deleted while it was in use.
func (r *PostgresAPIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID, lastUsedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at = $2 WHERE id = $1`, id, lastUsedAt)
	if err != nil {
		return fmt.Errorf("update api key last use: %w", err)
	}

	return nil
}

// Delete removes the record with the given ID of the given identity.
func (r *PostgresAPIKeyRepository) Delete(ctx context.Context, identityID, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM api_keys WHERE identity_id = $1 AND id = $2`, identityID, id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("delete api key: %w", ErrNotFound)
	}

	return nil
}

// IdentityExists reports whether the identity with the given ID exists.
func (r *PostgresAPIKeyRepository) IdentityExists(ctx context.Context, identityID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS (SELECT FROM identities WHERE id = $1)`, identityID)
	if err != nil {
		return false, fmt.Errorf("check api key identity: %w", err)
	}

	return exists, nil
}
