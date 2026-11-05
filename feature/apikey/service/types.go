package service

import (
	"time"
	"uuid"
)

// APIKey is a credential that authenticates requests as its identity. It doesn't hold the key itself,
// which is only known when the API key is created.
type APIKey struct {
	ID         uuid.UUID
	IdentityID uuid.UUID
	Name       string
	// KeySuffix is the key's last characters, which tell the holder of a key which API key it is.
	KeySuffix string
	// ExpiresAt is nil for an API key that never expires.
	ExpiresAt *time.Time
	// LastUsedAt is nil for an API key that has not been used yet. It's accurate to about a minute.
	LastUsedAt *time.Time
	// CreatedBy is the identity, or audit.SystemActor, that created the API key.
	CreatedBy uuid.UUID
	CreatedAt time.Time
}

// CreatedAPIKey is a new API key with its key, which can't be retrieved later.
type CreatedAPIKey struct {
	APIKey

	Key string
}

// APIKeyInput holds the API key fields set by callers when creating an API key.
type APIKeyInput struct {
	Name string
	// ExpiresAt is nil for an API key that never expires.
	ExpiresAt *time.Time
}
