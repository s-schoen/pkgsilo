// Package repository persists identities.
package repository

import (
	"context"
	"fmt"
	"uuid"

	"github.com/jmoiron/sqlx"
)

// IdentityRepository stores identity records.
type IdentityRepository interface {
	// Insert stores a new record and returns it as stored.
	Insert(ctx context.Context, rec IdentityRecord) (IdentityRecord, error)
	// Get returns the record with the given ID.
	Get(ctx context.Context, id uuid.UUID) (IdentityRecord, error)
	// List returns up to limit records, skipping the first offset, ordered by creation time.
	List(ctx context.Context, offset, limit int) ([]IdentityRecord, error)
	// Update overwrites the username, display name, UpdatedBy and UpdatedAt of the record with
	// rec.ID and returns it as stored. CreatedBy and CreatedAt are immutable and ignored.
	Update(ctx context.Context, rec IdentityRecord) (IdentityRecord, error)
	// Delete removes the record with the given ID.
	Delete(ctx context.Context, id uuid.UUID) error
}

const identityColumns = "id, username, display_name, created_by, updated_by, created_at, updated_at"

// PostgresIdentityRepository stores identity records in the identities table of PostgreSQL.
type PostgresIdentityRepository struct {
	db *sqlx.DB
}

var _ IdentityRepository = (*PostgresIdentityRepository)(nil)

func NewPostgresIdentityRepository(db *sqlx.DB) *PostgresIdentityRepository {
	return &PostgresIdentityRepository{db: db}
}

func (r *PostgresIdentityRepository) Insert(ctx context.Context, rec IdentityRecord) (IdentityRecord, error) {
	var stored IdentityRecord
	err := r.db.GetContext(ctx, &stored, `
		INSERT INTO identities (`+identityColumns+`)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+identityColumns,
		rec.ID, rec.Username, rec.DisplayName, rec.CreatedBy, rec.UpdatedBy, rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return IdentityRecord{}, fmt.Errorf("insert identity: %w", translateError(err))
	}

	return stored, nil
}

func (r *PostgresIdentityRepository) Get(ctx context.Context, id uuid.UUID) (IdentityRecord, error) {
	var rec IdentityRecord
	err := r.db.GetContext(ctx, &rec, `SELECT `+identityColumns+` FROM identities WHERE id = $1`, id)
	if err != nil {
		return IdentityRecord{}, fmt.Errorf("get identity: %w", translateError(err))
	}

	return rec, nil
}

func (r *PostgresIdentityRepository) List(ctx context.Context, offset, limit int) ([]IdentityRecord, error) {
	var recs []IdentityRecord
	err := r.db.SelectContext(ctx, &recs, `
		SELECT `+identityColumns+` FROM identities
		ORDER BY created_at, id
		LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list identities: %w", err)
	}

	return recs, nil
}

func (r *PostgresIdentityRepository) Update(ctx context.Context, rec IdentityRecord) (IdentityRecord, error) {
	var stored IdentityRecord
	err := r.db.GetContext(ctx, &stored, `
		UPDATE identities
		SET username = $2, display_name = $3, updated_by = $4, updated_at = $5
		WHERE id = $1
		RETURNING `+identityColumns,
		rec.ID, rec.Username, rec.DisplayName, rec.UpdatedBy, rec.UpdatedAt,
	)
	if err != nil {
		return IdentityRecord{}, fmt.Errorf("update identity: %w", translateError(err))
	}

	return stored, nil
}

func (r *PostgresIdentityRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM identities WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete identity: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete identity: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("delete identity: %w", ErrNotFound)
	}

	return nil
}
