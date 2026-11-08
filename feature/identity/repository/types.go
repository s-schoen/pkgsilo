package repository

import (
	"time"
	"uuid"
)

// IdentityRecord is the stored form of an identity, one row of the identities table.
type IdentityRecord struct {
	ID          uuid.UUID `db:"id"`
	Username    string    `db:"username"`
	DisplayName string    `db:"display_name"`
	CreatedBy   uuid.UUID `db:"created_by"`
	UpdatedBy   uuid.UUID `db:"updated_by"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// IdentityPatch changes some fields of an identity record: a nil field stays as it is.
type IdentityPatch struct {
	ID          uuid.UUID
	Username    *string
	DisplayName *string
	UpdatedBy   uuid.UUID
	UpdatedAt   time.Time
}

// MembershipRecord says that an identity is a member of a group, as far as identities need to know.
type MembershipRecord struct {
	IdentityID uuid.UUID `db:"identity_id"`
	GroupID    uuid.UUID `db:"group_id"`
}
