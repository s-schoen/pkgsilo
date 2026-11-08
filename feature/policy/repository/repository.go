// Package repository persists policies.
package repository

import (
	"context"
	"fmt"
	"uuid"

	"pkgsilo/core/database"
)

// PolicyRepository stores policy records. Every write takes part in a transaction of Write, which advances
// the policy revision with it.
type PolicyRepository interface {
	// Get returns the record with the given ID.
	Get(ctx context.Context, id uuid.UUID) (PolicyRecord, error)
	// ListAll returns all records, ordered by creation time.
	ListAll(ctx context.Context) ([]PolicyRecord, error)
	ReferenceLister
	// Write runs fn in a transaction that first advances the policy revision. This serializes policy
	// writes, and every instance loads the policies again on its first request after the commit. The
	// transaction commits if fn returns nil. Otherwise, it rolls back and Write returns an error wrapping fn's.
	Write(ctx context.Context, fn func(w PolicyWriter) error) error
}

// PolicyWriter reads and writes policy records in the transaction of PolicyRepository.Write.
type PolicyWriter interface {
	// Get returns the record with the given ID, like PolicyRepository.Get. Since Write serializes policy
	// writes, no other write changes it until the commit.
	Get(ctx context.Context, id uuid.UUID) (PolicyRecord, error)
	// ListAll returns all records, like PolicyRepository.ListAll.
	ListAll(ctx context.Context) ([]PolicyRecord, error)
	// ReferenceLister finds the objects that policies reference in the transaction, which holds a
	// connection of its own, so that it needs no second one.
	ReferenceLister
	// Insert stores a new record and returns it as stored.
	Insert(ctx context.Context, rec PolicyRecord) (PolicyRecord, error)
	// Update overwrites the name, source, UpdatedBy and UpdatedAt of the record with rec.ID and returns
	// it as stored. CreatedBy and CreatedAt are immutable and ignored.
	Update(ctx context.Context, rec PolicyRecord) (PolicyRecord, error)
	// Patch overwrites the fields that patch sets, UpdatedBy and UpdatedAt of the record with patch.ID and
	// returns it as stored. It writes no other column.
	Patch(ctx context.Context, patch PolicyPatch) (PolicyRecord, error)
	// Delete removes the record with the given ID.
	Delete(ctx context.Context, id uuid.UUID) error
}

// ReferenceLister finds the objects of other features that policies reference by ID, each kind in one query.
// The tables belong to their features.
type ReferenceLister interface {
	// ListExistingGroups returns those of the given group IDs that a group has.
	ListExistingGroups(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)
	// ListExistingIdentities returns those of the given identity IDs that an identity has.
	ListExistingIdentities(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)
}

const policyColumns = "id, name, source, created_by, updated_by, created_at, updated_at"

// PostgresPolicyRepository stores policy records in the policies table of PostgreSQL, and the revision of
// the policies in the policy_revision table.
type PostgresPolicyRepository struct {
	db database.Querier
}

var _ PolicyRepository = (*PostgresPolicyRepository)(nil)

// NewPostgresPolicyRepository returns a PostgresPolicyRepository that stores records in db. On a transaction,
// Write takes part in it instead of beginning one of its own, and its writes commit with the transaction.
func NewPostgresPolicyRepository(db database.Querier) *PostgresPolicyRepository {
	return &PostgresPolicyRepository{db: db}
}

// Get returns the record with the given ID.
func (r *PostgresPolicyRepository) Get(ctx context.Context, id uuid.UUID) (PolicyRecord, error) {
	return get(ctx, r.db, id)
}

// ListAll returns all records, ordered by creation time.
func (r *PostgresPolicyRepository) ListAll(ctx context.Context) ([]PolicyRecord, error) {
	return listAll(ctx, r.db)
}

// ListExistingGroups returns those of the given group IDs that a group has, in one query.
func (r *PostgresPolicyRepository) ListExistingGroups(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	return listExistingGroups(ctx, r.db, ids)
}

// ListExistingIdentities returns those of the given identity IDs that an identity has, in one query.
func (r *PostgresPolicyRepository) ListExistingIdentities(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	return listExistingIdentities(ctx, r.db, ids)
}

// Write runs fn in a transaction that first advances the policy revision. The transaction commits if fn
// returns nil. Otherwise, it rolls back and Write returns an error wrapping fn's. On a repository of a
// transaction, fn takes part in that one, which its owner commits or rolls back.
func (r *PostgresPolicyRepository) Write(ctx context.Context, fn func(w PolicyWriter) error) error {
	err := database.InTx(ctx, r.db, func(tx database.Querier) error {
		// The row lock that this takes holds every other policy write until the commit.
		if _, err := tx.ExecContext(ctx, `UPDATE policy_revision SET revision = revision + 1`); err != nil {
			return fmt.Errorf("advance policy revision: %w", err)
		}

		return fn(postgresPolicyWriter{q: tx})
	})
	if err != nil {
		return fmt.Errorf("write policies: %w", err)
	}

	return nil
}

// postgresPolicyWriter writes policy records in a transaction of PostgresPolicyRepository.Write.
type postgresPolicyWriter struct {
	q database.Querier
}

var _ PolicyWriter = postgresPolicyWriter{}

// Get returns the record with the given ID.
func (w postgresPolicyWriter) Get(ctx context.Context, id uuid.UUID) (PolicyRecord, error) {
	return get(ctx, w.q, id)
}

// ListAll returns all records, ordered by creation time.
func (w postgresPolicyWriter) ListAll(ctx context.Context) ([]PolicyRecord, error) {
	return listAll(ctx, w.q)
}

// ListExistingGroups returns those of the given group IDs that a group has, in one query.
func (w postgresPolicyWriter) ListExistingGroups(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	return listExistingGroups(ctx, w.q, ids)
}

// ListExistingIdentities returns those of the given identity IDs that an identity has, in one query.
func (w postgresPolicyWriter) ListExistingIdentities(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	return listExistingIdentities(ctx, w.q, ids)
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

// Patch overwrites the fields that patch sets, UpdatedBy and UpdatedAt of the record with patch.ID and
// returns it as stored. It writes no other column.
func (w postgresPolicyWriter) Patch(ctx context.Context, patch PolicyPatch) (PolicyRecord, error) {
	var stored PolicyRecord
	// COALESCE keeps a column whose field is nil, which only works while the columns are NOT NULL: a
	// nullable one needs to tell "unchanged" from "set to NULL" with a parameter of its own.
	err := w.q.GetContext(ctx, &stored, `
		UPDATE policies
		SET name = COALESCE($2::text, name),
			source = COALESCE($3::text, source),
			updated_by = $4, updated_at = $5
		WHERE id = $1
		RETURNING `+policyColumns,
		patch.ID, patch.Name, patch.Source, patch.UpdatedBy, patch.UpdatedAt,
	)
	if err != nil {
		return PolicyRecord{}, fmt.Errorf("patch policy: %w", translateError(err))
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

// get returns the record with the given ID that q finds.
func get(ctx context.Context, q database.Querier, id uuid.UUID) (PolicyRecord, error) {
	var rec PolicyRecord
	err := q.GetContext(ctx, &rec, `SELECT `+policyColumns+` FROM policies WHERE id = $1`, id)
	if err != nil {
		return PolicyRecord{}, fmt.Errorf("get policy: %w", translateError(err))
	}

	return rec, nil
}

// listAll returns all records that q finds, ordered by creation time.
func listAll(ctx context.Context, q database.Querier) ([]PolicyRecord, error) {
	var recs []PolicyRecord
	err := q.SelectContext(ctx, &recs, `
		SELECT `+policyColumns+` FROM policies
		ORDER BY created_at, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all policies: %w", err)
	}

	return recs, nil
}

// listExistingGroups returns those of the given group IDs that q finds a group with.
func listExistingGroups(ctx context.Context, q database.Querier, ids []uuid.UUID) ([]uuid.UUID, error) {
	var existing []uuid.UUID
	if err := q.SelectContext(ctx, &existing, `SELECT id FROM groups WHERE id = ANY($1)`, ids); err != nil {
		return nil, fmt.Errorf("list existing groups: %w", err)
	}

	return existing, nil
}

// listExistingIdentities returns those of the given identity IDs that q finds an identity with.
func listExistingIdentities(ctx context.Context, q database.Querier, ids []uuid.UUID) ([]uuid.UUID, error) {
	var existing []uuid.UUID
	if err := q.SelectContext(ctx, &existing, `SELECT id FROM identities WHERE id = ANY($1)`, ids); err != nil {
		return nil, fmt.Errorf("list existing identities: %w", err)
	}

	return existing, nil
}
