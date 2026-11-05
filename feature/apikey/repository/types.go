package repository

import (
	"time"
	"uuid"
)

// APIKeyRecord is the stored form of an API key, one row of the api_keys table.
type APIKeyRecord struct {
	ID         uuid.UUID `db:"id"`
	IdentityID uuid.UUID `db:"identity_id"`
	Name       string    `db:"name"`
	KeyHash    []byte    `db:"key_hash"`
	KeySuffix  string    `db:"key_suffix"`
	// ExpiresAt is nil for a key that never expires.
	ExpiresAt *time.Time `db:"expires_at"`
	// LastUsedAt is nil for a key that has not been used yet.
	LastUsedAt *time.Time `db:"last_used_at"`
	CreatedBy  uuid.UUID  `db:"created_by"`
	CreatedAt  time.Time  `db:"created_at"`
}
