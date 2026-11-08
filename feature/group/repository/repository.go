// Package repository persists groups and their members.
package repository

import (
	"context"
	"fmt"
	"uuid"

	"pkgsilo/core/database"
)

// GroupRepository stores group records and the member records of each group.
type GroupRepository interface {
	// Insert stores a new record and returns it as stored.
	Insert(ctx context.Context, rec GroupRecord) (GroupRecord, error)
	// Get returns the record with the given ID.
	Get(ctx context.Context, id uuid.UUID) (GroupRecord, error)
	// ListAll returns all records, ordered by creation time.
	ListAll(ctx context.Context) ([]GroupRecord, error)
	// Update overwrites the name, UpdatedBy and UpdatedAt of the record with rec.ID and returns it
	// as stored. CreatedBy and CreatedAt are immutable and ignored.
	Update(ctx context.Context, rec GroupRecord) (GroupRecord, error)
	// Patch overwrites the fields that patch sets, UpdatedBy and UpdatedAt of the record with patch.ID and
	// returns it as stored. It writes no other column, so that concurrent patches of other fields are kept.
	Patch(ctx context.Context, patch GroupPatch) (GroupRecord, error)
	// Delete removes the record with the given ID and its member records.
	Delete(ctx context.Context, id uuid.UUID) error
	// InsertMember stores a new member record. If the group already has a member record for the
	// identity, it's kept unchanged.
	InsertMember(ctx context.Context, rec MemberRecord) error
	// ListMembers returns up to limit member records of the group, skipping the first offset,
	// ordered by creation time.
	ListMembers(ctx context.Context, groupID uuid.UUID, offset, limit int) ([]MemberRecord, error)
	// DeleteMember removes the member record of the identity from the group.
	DeleteMember(ctx context.Context, groupID, identityID uuid.UUID) error
	// ListIdentityGroups returns the IDs of the groups that the identity is a member of, in no particular
	// order. An identity that doesn't exist is a member of none.
	ListIdentityGroups(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error)
}

const (
	groupColumns  = "id, name, created_by, updated_by, created_at, updated_at"
	memberColumns = "group_id, identity_id, created_by, created_at"
)

// PostgresGroupRepository stores group records in the groups table of PostgreSQL and their member
// records in the group_members table.
type PostgresGroupRepository struct {
	db database.Querier
}

var _ GroupRepository = (*PostgresGroupRepository)(nil)

// NewPostgresGroupRepository returns a PostgresGroupRepository that stores records in db.
func NewPostgresGroupRepository(db database.Querier) *PostgresGroupRepository {
	return &PostgresGroupRepository{db: db}
}

// Insert stores a new record and returns it as stored.
func (r *PostgresGroupRepository) Insert(ctx context.Context, rec GroupRecord) (GroupRecord, error) {
	var stored GroupRecord
	err := r.db.GetContext(ctx, &stored, `
		INSERT INTO groups (`+groupColumns+`)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+groupColumns,
		rec.ID, rec.Name, rec.CreatedBy, rec.UpdatedBy, rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return GroupRecord{}, fmt.Errorf("insert group: %w", translateError(err))
	}

	return stored, nil
}

// Get returns the record with the given ID.
func (r *PostgresGroupRepository) Get(ctx context.Context, id uuid.UUID) (GroupRecord, error) {
	var rec GroupRecord
	err := r.db.GetContext(ctx, &rec, `SELECT `+groupColumns+` FROM groups WHERE id = $1`, id)
	if err != nil {
		return GroupRecord{}, fmt.Errorf("get group: %w", translateError(err))
	}

	return rec, nil
}

// ListAll returns all records, ordered by creation time.
func (r *PostgresGroupRepository) ListAll(ctx context.Context) ([]GroupRecord, error) {
	var recs []GroupRecord
	err := r.db.SelectContext(ctx, &recs, `
		SELECT `+groupColumns+` FROM groups
		ORDER BY created_at, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all groups: %w", err)
	}

	return recs, nil
}

// Update overwrites the name, UpdatedBy and UpdatedAt of the record with rec.ID and returns it
// as stored. CreatedBy and CreatedAt are immutable and ignored.
func (r *PostgresGroupRepository) Update(ctx context.Context, rec GroupRecord) (GroupRecord, error) {
	var stored GroupRecord
	err := r.db.GetContext(ctx, &stored, `
		UPDATE groups
		SET name = $2, updated_by = $3, updated_at = $4
		WHERE id = $1
		RETURNING `+groupColumns,
		rec.ID, rec.Name, rec.UpdatedBy, rec.UpdatedAt,
	)
	if err != nil {
		return GroupRecord{}, fmt.Errorf("update group: %w", translateError(err))
	}

	return stored, nil
}

// Patch overwrites the fields that patch sets, UpdatedBy and UpdatedAt of the record with patch.ID and
// returns it as stored. It writes no other column, so that concurrent patches of other fields are kept.
func (r *PostgresGroupRepository) Patch(ctx context.Context, patch GroupPatch) (GroupRecord, error) {
	var stored GroupRecord
	// COALESCE keeps a column whose field is nil, which only works while the columns are NOT NULL: a
	// nullable one needs to tell "unchanged" from "set to NULL" with a parameter of its own.
	err := r.db.GetContext(ctx, &stored, `
		UPDATE groups
		SET name = COALESCE($2::text, name), updated_by = $3, updated_at = $4
		WHERE id = $1
		RETURNING `+groupColumns,
		patch.ID, patch.Name, patch.UpdatedBy, patch.UpdatedAt,
	)
	if err != nil {
		return GroupRecord{}, fmt.Errorf("patch group: %w", translateError(err))
	}

	return stored, nil
}

// Delete removes the record with the given ID and, by the foreign key's cascade, its member records.
func (r *PostgresGroupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM groups WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("delete group: %w", ErrNotFound)
	}

	return nil
}

// InsertMember stores a new member record. If the group already has a member record for the
// identity, it's kept unchanged.
func (r *PostgresGroupRepository) InsertMember(ctx context.Context, rec MemberRecord) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO group_members (`+memberColumns+`)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (group_id, identity_id) DO NOTHING`,
		rec.GroupID, rec.IdentityID, rec.CreatedBy, rec.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert group member: %w", translateError(err))
	}

	return nil
}

// ListMembers returns up to limit member records of the group, skipping the first offset,
// ordered by creation time.
func (r *PostgresGroupRepository) ListMembers(
	ctx context.Context, groupID uuid.UUID, offset, limit int,
) ([]MemberRecord, error) {
	var recs []MemberRecord
	err := r.db.SelectContext(ctx, &recs, `
		SELECT `+memberColumns+` FROM group_members
		WHERE group_id = $1
		ORDER BY created_at, identity_id
		LIMIT $2 OFFSET $3`,
		groupID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}

	return recs, nil
}

// DeleteMember removes the member record of the identity from the group.
func (r *PostgresGroupRepository) DeleteMember(ctx context.Context, groupID, identityID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND identity_id = $2`, groupID, identityID)
	if err != nil {
		return fmt.Errorf("delete group member: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete group member: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("delete group member: %w", ErrMemberNotFound)
	}

	return nil
}

// ListIdentityGroups returns the IDs of the groups that the identity is a member of, in no particular
// order. An identity that doesn't exist is a member of none.
func (r *PostgresGroupRepository) ListIdentityGroups(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error) {
	var groups []uuid.UUID
	err := r.db.SelectContext(ctx, &groups, `SELECT group_id FROM group_members WHERE identity_id = $1`, identityID)
	if err != nil {
		return nil, fmt.Errorf("list identity groups: %w", err)
	}

	return groups, nil
}
