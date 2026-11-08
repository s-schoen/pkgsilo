// Package repository persists identities.
package repository

import (
	"context"
	"fmt"
	"uuid"

	"pkgsilo/core/database"
	"pkgsilo/core/pagination"
)

// IdentityRepository stores identity records.
type IdentityRepository interface {
	// Insert stores a new record and returns it as stored.
	Insert(ctx context.Context, rec IdentityRecord) (IdentityRecord, error)
	// Get returns the record with the given ID.
	Get(ctx context.Context, id uuid.UUID) (IdentityRecord, error)
	// ListAll returns all records, ordered by creation time. It returns an error wrapping
	// ErrTooManyRows if there are more than pagination.MaxAllRows.
	ListAll(ctx context.Context) ([]IdentityRecord, error)
	// ListMemberships returns the group memberships of the identities with the given IDs, in one query.
	ListMemberships(ctx context.Context, identityIDs []uuid.UUID) ([]MembershipRecord, error)
	// Update overwrites the username, display name, UpdatedBy and UpdatedAt of the record with
	// rec.ID and returns it as stored. CreatedBy and CreatedAt are immutable and ignored.
	Update(ctx context.Context, rec IdentityRecord) (IdentityRecord, error)
	// Patch overwrites the fields that patch sets, UpdatedBy and UpdatedAt of the record with patch.ID and
	// returns it as stored. It writes no other column, so that concurrent patches of other fields are kept.
	Patch(ctx context.Context, patch IdentityPatch) (IdentityRecord, error)
	// Delete removes the record with the given ID.
	Delete(ctx context.Context, id uuid.UUID) error
}

const identityColumns = "id, username, display_name, created_by, updated_by, created_at, updated_at"

// PostgresIdentityRepository stores identity records in the identities table of PostgreSQL.
type PostgresIdentityRepository struct {
	db database.Querier
}

var _ IdentityRepository = (*PostgresIdentityRepository)(nil)

// NewPostgresIdentityRepository returns a PostgresIdentityRepository that stores records in db.
func NewPostgresIdentityRepository(db database.Querier) *PostgresIdentityRepository {
	return &PostgresIdentityRepository{db: db}
}

// Insert stores a new record and returns it as stored.
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

// Get returns the record with the given ID.
func (r *PostgresIdentityRepository) Get(ctx context.Context, id uuid.UUID) (IdentityRecord, error) {
	var rec IdentityRecord
	err := r.db.GetContext(ctx, &rec, `SELECT `+identityColumns+` FROM identities WHERE id = $1`, id)
	if err != nil {
		return IdentityRecord{}, fmt.Errorf("get identity: %w", translateError(err))
	}

	return rec, nil
}

// ListAll returns all records, ordered by creation time. It returns an error wrapping ErrTooManyRows if
// there are more than pagination.MaxAllRows.
func (r *PostgresIdentityRepository) ListAll(ctx context.Context) ([]IdentityRecord, error) {
	var recs []IdentityRecord
	err := r.db.SelectContext(ctx, &recs, `
		SELECT `+identityColumns+` FROM identities
		ORDER BY created_at, id
		LIMIT $1`,
		pagination.MaxAllRows+1,
	)
	if err != nil {
		return nil, fmt.Errorf("list all identities: %w", err)
	}
	if len(recs) > pagination.MaxAllRows {
		return nil, fmt.Errorf("list all identities: over %d: %w", pagination.MaxAllRows, ErrTooManyRows)
	}

	return recs, nil
}

// ListMemberships returns the group memberships of the identities with the given IDs, in one query.
func (r *PostgresIdentityRepository) ListMemberships(
	ctx context.Context, identityIDs []uuid.UUID,
) ([]MembershipRecord, error) {
	var recs []MembershipRecord
	err := r.db.SelectContext(ctx, &recs,
		`SELECT identity_id, group_id FROM group_members WHERE identity_id = ANY($1)`, identityIDs)
	if err != nil {
		return nil, fmt.Errorf("list identity memberships: %w", err)
	}

	return recs, nil
}

// Update overwrites the username, display name, UpdatedBy and UpdatedAt of the record with
// rec.ID and returns it as stored. CreatedBy and CreatedAt are immutable and ignored.
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

// Patch overwrites the fields that patch sets, UpdatedBy and UpdatedAt of the record with patch.ID and
// returns it as stored. It writes no other column, so that concurrent patches of other fields are kept.
func (r *PostgresIdentityRepository) Patch(ctx context.Context, patch IdentityPatch) (IdentityRecord, error) {
	var stored IdentityRecord
	// COALESCE keeps a column whose field is nil, which only works while the columns are NOT NULL: a
	// nullable one needs to tell "unchanged" from "set to NULL" with a parameter of its own.
	err := r.db.GetContext(ctx, &stored, `
		UPDATE identities
		SET username = COALESCE($2::text, username),
			display_name = COALESCE($3::text, display_name),
			updated_by = $4, updated_at = $5
		WHERE id = $1
		RETURNING `+identityColumns,
		patch.ID, patch.Username, patch.DisplayName, patch.UpdatedBy, patch.UpdatedAt,
	)
	if err != nil {
		return IdentityRecord{}, fmt.Errorf("patch identity: %w", translateError(err))
	}

	return stored, nil
}

// Delete removes the record with the given ID.
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
