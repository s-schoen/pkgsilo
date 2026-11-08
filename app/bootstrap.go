package app

import (
	"context"
	"fmt"
	"uuid"

	"github.com/jmoiron/sqlx"

	"pkgsilo/core/audit"
	"pkgsilo/core/authz"
	apikeyrepo "pkgsilo/feature/apikey/repository"
	apikeyservice "pkgsilo/feature/apikey/service"
	grouprepo "pkgsilo/feature/group/repository"
	groupservice "pkgsilo/feature/group/service"
	identityrepo "pkgsilo/feature/identity/repository"
	identityservice "pkgsilo/feature/identity/service"
)

const (
	adminUsername    = "admin"
	adminDisplayName = "Administrator"
	bootstrapKeyName = "bootstrap"
)

// administratorsGroupID is the ID of the group that the migrations create, and that their administrators
// policy grants everything.
var administratorsGroupID = uuid.MustParse("7c1d3e52-8a4f-4b6e-9f21-5d0c8e7a3b14")

// Bootstrap makes a database without identities usable: without an identity, no request can
// authenticate to create the first one. In one transaction, it creates the identity admin as a member of
// the administrators group, and an API key of it that never expires, hashed under the secret
// apiKeyPepper. It returns the key, which can't be retrieved later, for the caller to hand to the
// operator. It returns an empty string if identities already exist.
//
// Bootstrap runs without a caller, so that nothing limits what it may do. It returns an error if the
// administrators group was deleted, since an admin without it could do nothing.
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

	identities := identityservice.NewIdentityService(identityrepo.NewPostgresIdentityRepository(tx), authz.AllowAll())
	existing, err := identities.List(ctx, audit.SystemActor(), 0, 1)
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

	groups := groupservice.NewGroupService(grouprepo.NewPostgresGroupRepository(tx), authz.AllowAll())
	if err := groups.AddMember(ctx, audit.SystemActor(), administratorsGroupID, admin.ID); err != nil {
		return "", fmt.Errorf("add admin to administrators: %w", err)
	}

	apiKeys := apikeyservice.NewAPIKeyService(apikeyrepo.NewPostgresAPIKeyRepository(tx), authz.AllowAll(), apiKeyPepper)
	apiKey, err := apiKeys.Create(ctx, audit.SystemActor(), admin.ID, apikeyservice.APIKeyInput{Name: bootstrapKeyName})
	if err != nil {
		return "", fmt.Errorf("create admin api key: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit transaction: %w", err)
	}

	return apiKey.Key, nil
}
