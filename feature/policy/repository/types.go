package repository

import (
	"time"
	"uuid"
)

// PolicyRecord is the stored form of a policy, one row of the policies table.
type PolicyRecord struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	Source    string    `db:"source"`
	CreatedBy uuid.UUID `db:"created_by"`
	UpdatedBy uuid.UUID `db:"updated_by"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
