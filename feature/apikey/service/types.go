package service

import (
	"time"
	"uuid"

	"pkgsilo/core/authz"
)

// APIKey is a credential that authenticates requests as its identity, as an actor sees it: every field but
// the ID is hidden unless the actor may read it. It doesn't hold the key itself, which is only known when
// the API key is created. The authz tag of a field names its action, such as apikey:read:name.
type APIKey struct {
	ID         uuid.UUID
	IdentityID authz.Field[uuid.UUID] `authz:"identityId"`
	Name       authz.Field[string]    `authz:"name"`
	// KeySuffix is the key's last characters, which tell the holder of a key which API key it is.
	KeySuffix authz.Field[string] `authz:"keySuffix"`
	// ExpiresAt is nil for an API key that never expires.
	ExpiresAt authz.Field[*time.Time] `authz:"expiresAt"`
	// LastUsedAt is nil for an API key that has not been used yet. It's accurate to about a minute.
	LastUsedAt authz.Field[*time.Time] `authz:"lastUsedAt"`
	// CreatedBy is the identity, or audit.SystemActor, that created the API key.
	CreatedBy authz.Field[uuid.UUID] `authz:"createdBy"`
	CreatedAt authz.Field[time.Time] `authz:"createdAt"`
}

// AuthenticatedKey is the API key that a presented key belongs to, as far as a request needs it once it's
// authenticated.
type AuthenticatedKey struct {
	ID uuid.UUID
	// IdentityID is the identity that the request acts as.
	IdentityID uuid.UUID
}

// CreatedAPIKey is a new API key with its key, which can't be retrieved later. The key is never hidden,
// since the API key would be useless without it.
type CreatedAPIKey struct {
	APIKey

	Key string
}

// APIKeyInput holds the API key fields set by callers when creating an API key. The authz tag of a field
// names its action, such as apikey:create:name.
type APIKeyInput struct {
	Name string `authz:"name"`
	// ExpiresAt is nil for an API key that never expires.
	ExpiresAt *time.Time `authz:"expiresAt"`
}
