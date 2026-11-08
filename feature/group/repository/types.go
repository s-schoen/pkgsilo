package repository

import (
	"time"
	"uuid"
)

// GroupRecord is the stored form of a group, one row of the groups table.
type GroupRecord struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	CreatedBy uuid.UUID `db:"created_by"`
	UpdatedBy uuid.UUID `db:"updated_by"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// GroupPatch changes some fields of a group record: a nil field stays as it is.
type GroupPatch struct {
	ID        uuid.UUID
	Name      *string
	UpdatedBy uuid.UUID
	UpdatedAt time.Time
}

// MemberRecord is the stored membership of an identity in a group, one row of the group_members table.
type MemberRecord struct {
	GroupID    uuid.UUID `db:"group_id"`
	IdentityID uuid.UUID `db:"identity_id"`
	CreatedBy  uuid.UUID `db:"created_by"`
	CreatedAt  time.Time `db:"created_at"`
}
