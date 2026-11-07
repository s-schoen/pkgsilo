// Package repository persists policies.
package repository

import (
	"context"
	"fmt"
	"uuid"

	"github.com/jmoiron/sqlx"

	"pkgsilo/core/database"
	"pkgsilo/core/pagination"
)

// PolicyRepository stores policy records. Every write takes part in a transaction of Write, which advances
// the policy revision with it.
type PolicyRepository interface {
	// Get returns the record with the given ID.
	Get(ctx context.Context, id uuid.UUID) (PolicyRecord, error)
	// ListAll returns all records, ordered by creation time. It returns an error wrapping ErrTooManyRows
	// if there are more than pagination.MaxAllRows.
	ListAll(ctx context.Context) ([]PolicyRecord, error)
	// ListExistingGroups returns those of the given group IDs that a group has, in one query.
	ListExistingGroups(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)
	// Write runs fn in a transaction that first advances the policy revision. This serializes policy
	// writes, and every instance loads the policies again on its first request after the commit. The
	// transaction commits if fn returns nil. Otherwise, it rolls back and Write returns fn's error.
	Write(ctx context.Context, fn func(w PolicyWriter) error) error
}

// PolicyWriter reads and writes policy records in the transaction of PolicyRepository.Write.
type PolicyWriter interface {
	// ListAll returns all records, like PolicyRepository.ListAll.
	ListAll(ctx context.Context) ([]PolicyRecord, error)
	// Insert stores a new record and returns it as stored.
	Insert(ctx context.Context, rec PolicyRecord) (PolicyRecord, error)
	// Update overwrites the name, source, UpdatedBy and UpdatedAt of the record with rec.ID and returns
	// it as stored. CreatedBy and CreatedAt are immutable and ignored.
	Update(ctx context.Context, rec PolicyRecord) (PolicyRecord, error)
	// Delete removes the record with the given ID.
	Delete(ctx context.Context, id uuid.UUID) error
}

const policyColumns = "id, name, source, created_by, updated_by, created_at, updated_at"

// PostgresPolicyRepository stores policy records in the policies table of PostgreSQL, and the revision of
// the policies in the policy_revision table.
type PostgresPolicyRepository struct {
	db *sqlx.DB
}

var _ PolicyRepository = (*PostgresPolicyRepository)(nil)

// NewPostgresPolicyRepository returns a PostgresPolicyRepository that stores records in db.
func NewPostgresPolicyRepository(db *sqlx.DB) *PostgresPolicyRepository {
	return &PostgresPolicyRepository{db: db}
}

// Get returns the record with the given ID.
func (r *PostgresPolicyRepository) Get(ctx context.Context, id uuid.UUID) (PolicyRecord, error) {
	var rec PolicyRecord
	err := r.db.GetContext(ctx, &rec, `SELECT `+policyColumns+` FROM policies WHERE id = $1`, id)
	if err != nil {
		return PolicyRecord{}, fmt.Errorf("get policy: %w", translateError(err))
	}

	return rec, nil
}

// ListAll returns all records, ordered by creation time. It returns an error wrapping ErrTooManyRows if
// there are more than pagination.MaxAllRows.
func (r *PostgresPolicyRepository) ListAll(ctx context.Context) ([]PolicyRecord, error) {
	return listAll(ctx, r.db)
}

// ListExistingGroups returns those of the given group IDs that a group has, in one query.
func (r *PostgresPolicyRepository) ListExistingGroups(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	var existing []uuid.UUID
	if err := r.db.SelectContext(ctx, &existing, `SELECT id FROM groups WHERE id = ANY($1)`, ids); err != nil {
		return nil, fmt.Errorf("list existing groups: %w", err)
	}

	return existing, nil
}

// Write runs fn in a transaction that first advances the policy revision. The transaction commits if fn
// returns nil. Otherwise, it rolls back and Write returns fn's error.
func (r *PostgresPolicyRepository) Write(ctx context.Context, fn func(w PolicyWriter) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin policy write: %w", err)
	}
	// Does nothing once the transaction is committed.
	defer func() { _ = tx.Rollback() }()

	// The row lock that this takes holds every other policy write until the commit.
	if _, err := tx.ExecContext(ctx, `UPDATE policy_revision SET revision = revision + 1`); err != nil {
		return fmt.Errorf("advance policy revision: %w", err)
	}

	if err := fn(postgresPolicyWriter{q: tx}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit policy write: %w", err)
	}

	return nil
}

// postgresPolicyWriter writes policy records in a transaction of PostgresPolicyRepository.Write.
type postgresPolicyWriter struct {
	q database.Querier
}

var _ PolicyWriter = postgresPolicyWriter{}

// ListAll returns all records, ordered by creation time.
func (w postgresPolicyWriter) ListAll(ctx context.Context) ([]PolicyRecord, error) {
	return listAll(ctx, w.q)
}

// Insert stores a new record and returns it as stored.
func (w postgresPolicyWriter) Insert(ctx context.Context, rec PolicyRecord) (PolicyRecord, error) {
	var stored PolicyRecord
	err := w.q.GetContext(ctx, &stored, `
		INSERT INTO policies (`+policyColumns+`)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+policyColumns,
		rec.ID, rec.Name, rec.Source, rec.CreatedBy, rec.UpdatedBy, rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return PolicyRecord{}, fmt.Errorf("insert policy: %w", translateError(err))
	}

	return stored, nil
}

// Update overwrites the name, source, UpdatedBy and UpdatedAt of the record with rec.ID and returns it as
// stored. CreatedBy and CreatedAt are immutable and ignored.
func (w postgresPolicyWriter) Update(ctx context.Context, rec PolicyRecord) (PolicyRecord, error) {
	var stored PolicyRecord
	err := w.q.GetContext(ctx, &stored, `
		UPDATE policies
		SET name = $2, source = $3, updated_by = $4, updated_at = $5
		WHERE id = $1
		RETURNING `+policyColumns,
		rec.ID, rec.Name, rec.Source, rec.UpdatedBy, rec.UpdatedAt,
	)
	if err != nil {
		return PolicyRecord{}, fmt.Errorf("update policy: %w", translateError(err))
	}

	return stored, nil
}

// Delete removes the record with the given ID.
func (w postgresPolicyWriter) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := w.q.ExecContext(ctx, `DELETE FROM policies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete policy: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("delete policy: %w", ErrNotFound)
	}

	return nil
}

// listAll returns all records that q finds, ordered by creation time. It returns an error wrapping
// ErrTooManyRows if there are more than pagination.MaxAllRows.
func listAll(ctx context.Context, q database.Querier) ([]PolicyRecord, error) {
	var recs []PolicyRecord
	err := q.SelectContext(ctx, &recs, `
		SELECT `+policyColumns+` FROM policies
		ORDER BY created_at, id
		LIMIT $1`,
		pagination.MaxAllRows+1,
	)
	if err != nil {
		return nil, fmt.Errorf("list all policies: %w", err)
	}
	if len(recs) > pagination.MaxAllRows {
		return nil, fmt.Errorf("list all policies: over %d: %w", pagination.MaxAllRows, ErrTooManyRows)
	}

	return recs, nil
}
