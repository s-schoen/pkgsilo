package app

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"pkgsilo/core/audit"
	"pkgsilo/core/authz"
	apikeyrepo "pkgsilo/feature/apikey/repository"
	apikeyservice "pkgsilo/feature/apikey/service"
	grouprepo "pkgsilo/feature/group/repository"
	groupservice "pkgsilo/feature/group/service"
	identityrepo "pkgsilo/feature/identity/repository"
	identityservice "pkgsilo/feature/identity/service"
	policyrepo "pkgsilo/feature/policy/repository"
	policyservice "pkgsilo/feature/policy/service"
)

const (
	adminUsername            = "admin"
	adminDisplayName         = "Administrator"
	administratorsGroupName  = "administrators"
	administratorsPolicyName = "administrators"
	bootstrapKeyName         = "bootstrap"
)

// Bootstrap makes a database without identities usable: without an identity, no request can
// authenticate to create the first one. In one transaction, it creates the identity admin, the group
// administrators with admin as its member, the policy administrators that grants the group everything,
// and an API key of admin that never expires, hashed under the secret apiKeyPepper. The group and the
// policy are like any other afterwards: nothing refers to them but each other. It returns the key, which
// can't be retrieved later, for the caller to hand to the operator. It returns an empty string if
// identities already exist.
//
// Bootstrap runs without a caller, so that nothing limits what it may do. It returns an error wrapping
// groupservice.ErrGroupNameTaken or policyservice.ErrPolicyNameTaken, without leaving anything behind, if
// a group or policy named administrators already exists, as it does when every identity was deleted after
// the first start: an operator deletes or renames it first.
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
	administrators, err := groups.Create(ctx, audit.SystemActor(), groupservice.GroupInput{Name: administratorsGroupName})
	if err != nil {
		return "", fmt.Errorf("create administrators group: %w", err)
	}
	if err := groups.AddMember(ctx, audit.SystemActor(), administrators.ID, admin.ID); err != nil {
		return "", fmt.Errorf("add admin to administrators group: %w", err)
	}

	policies := policyservice.NewPolicyService(policyrepo.NewPostgresPolicyRepository(tx), authz.AllowAll(), schema())
	_, err = policies.Create(ctx, audit.SystemActor(), policyservice.PolicyInput{
		Name:   administratorsPolicyName,
		Source: fmt.Sprintf(`permit (principal in Group::%q, action, resource);`, administrators.ID),
	})
	if err != nil {
		return "", fmt.Errorf("create administrators policy: %w", err)
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
