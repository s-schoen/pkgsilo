package app

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"pkgsilo/core/audit"
	apikeyrepo "pkgsilo/feature/apikey/repository"
	apikeyservice "pkgsilo/feature/apikey/service"
	identityrepo "pkgsilo/feature/identity/repository"
	identityservice "pkgsilo/feature/identity/service"
)

const (
	adminUsername    = "admin"
	adminDisplayName = "Administrator"
	bootstrapKeyName = "bootstrap"
)

// Bootstrap makes a database without identities usable: without an identity, no request can
// authenticate to create the first one. In one transaction, it creates the identity admin and an API
// key of it that never expires, hashed under the secret apiKeyPepper. It returns the key, which can't
// be retrieved later, for the caller to hand to the operator. It returns an empty string if identities
// already exist.
//
// Instances that start at once may all find no identities. All but one then fail on the unique
// username and return an error, without leaving anything behind: after a restart, they find the admin.
func Bootstrap(ctx context.Context, db *sqlx.DB, apiKeyPepper []byte) (string, error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	// Does nothing once the transaction is committed.
	defer func() { _ = tx.Rollback() }()

	identities := identityservice.NewIdentityService(identityrepo.NewPostgresIdentityRepository(tx))
	existing, err := identities.List(ctx, 0, 1)
	if err != nil {
		return "", fmt.Errorf("find identities: %w", err)
	}
	if len(existing.Items) > 0 {
		return "", nil
	}

	admin, err := identities.Create(ctx, audit.SystemActor(), identityservice.IdentityInput{
		Username:    adminUsername,
		DisplayName: adminDisplayName,
	})
	if err != nil {
		return "", fmt.Errorf("create admin: %w", err)
	}

	apiKeys := apikeyservice.NewAPIKeyService(apikeyrepo.NewPostgresAPIKeyRepository(tx), apiKeyPepper)
	apiKey, err := apiKeys.Create(ctx, audit.SystemActor(), admin.ID, apikeyservice.APIKeyInput{Name: bootstrapKeyName})
	if err != nil {
		return "", fmt.Errorf("create admin api key: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit transaction: %w", err)
	}

	return apiKey.Key, nil
}
