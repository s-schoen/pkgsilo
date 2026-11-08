package app

import (
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"

	"pkgsilo/core/audit"
	"pkgsilo/core/authz"
	apikeyrepo "pkgsilo/feature/apikey/repository"
	apikeyservice "pkgsilo/feature/apikey/service"
	groupservice "pkgsilo/feature/group/service"
	identityrepo "pkgsilo/feature/identity/repository"
	identityservice "pkgsilo/feature/identity/service"
	policyservice "pkgsilo/feature/policy/service"
	"pkgsilo/test/databasetest"
)

var testPepper = []byte("test-pepper-of-at-least-32-characters")

// countRows returns the number of rows of the table, whose name must be safe to format into a query.
func countRows(t *testing.T, db *sqlx.DB, table string) int {
	t.Helper()

	var count int
	require.NoError(t, db.GetContext(t.Context(), &count, "SELECT count(*) FROM "+table))

	return count
}

// countAdministratorsPolicies returns the number of policies named administrators.
func countAdministratorsPolicies(t *testing.T, db *sqlx.DB) int {
	t.Helper()

	var count int
	require.NoError(t, db.GetContext(t.Context(), &count, "SELECT count(*) FROM policies WHERE name = 'administrators'"))

	return count
}

// administratorsGroup returns the ID of the group named administrators.
func administratorsGroup(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	require.NoError(t, db.GetContext(t.Context(), &id, "SELECT id FROM groups WHERE name = 'administrators'"))

	return id
}

// groupMembers returns the identities that are members of the group, in no particular order.
func groupMembers(t *testing.T, db *sqlx.DB, group uuid.UUID) []uuid.UUID {
	t.Helper()

	var members []uuid.UUID
	require.NoError(t, db.SelectContext(t.Context(), &members,
		"SELECT identity_id FROM group_members WHERE group_id = $1", group))

	return members
}

func TestBootstrap_FirstStart(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)

	key, err := Bootstrap(t.Context(), db, testPepper)
	require.NoError(t, err)

	identities := identityservice.NewIdentityService(identityrepo.NewPostgresIdentityRepository(db), authz.AllowAll())
	page, err := identities.List(t.Context(), audit.SystemActor(), 0, 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	admin := page.Items[0]
	assert.Equal(t, authz.Visible("admin"), admin.Username)
	assert.Equal(t, authz.Visible("Administrator"), admin.DisplayName)
	assert.Equal(t, authz.Visible(audit.SystemActor()), admin.CreatedBy)
	assertAdministrator(t, db, admin.ID)

	apiKeys := apikeyservice.NewAPIKeyService(apikeyrepo.NewPostgresAPIKeyRepository(db), authz.AllowAll(), testPepper)
	authenticated, err := apiKeys.Authenticate(t.Context(), key)
	require.NoError(t, err, "the returned key must authenticate")
	assert.Equal(t, admin.ID, authenticated.IdentityID)
	apiKey, err := apiKeys.Get(t.Context(), audit.SystemActor(), admin.ID, authenticated.ID)
	require.NoError(t, err)
	assert.Equal(t, authz.Visible("bootstrap"), apiKey.Name)
	assert.Equal(t, authz.Visible[*time.Time](nil), apiKey.ExpiresAt, "the first key must never expire")
	assert.Equal(t, authz.Visible(audit.SystemActor()), apiKey.CreatedBy)
}

// assertAdministrator asserts that the identity is the only member of the group administrators, which the
// policy administrators grants every action on every resource.
func assertAdministrator(t *testing.T, db *sqlx.DB, identity uuid.UUID) {
	t.Helper()

	group := administratorsGroup(t, db)
	assert.Equal(t, []uuid.UUID{identity}, groupMembers(t, db, group))

	var source string
	require.NoError(t, db.GetContext(t.Context(), &source, "SELECT source FROM policies WHERE name = 'administrators'"))
	assert.Equal(t, `permit (principal in Group::"`+group.String()+`", action, resource);`, source)

	decider, err := authz.New(db, noop.NewMeterProvider()).For(t.Context(), identity)
	require.NoError(t, err)
	can, err := decider.Can(authz.NewAction("other:anything"), authz.Resource{Type: "Other", ID: uuid.NewV4()})
	require.NoError(t, err)
	assert.True(t, can, "the administrators policy must grant everything")
}

func TestBootstrap_AlreadyBootstrapped(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	_, err := Bootstrap(t.Context(), db, testPepper)
	require.NoError(t, err)

	key, err := Bootstrap(t.Context(), db, testPepper)
	require.NoError(t, err)

	assert.Empty(t, key)
	assert.Equal(t, 1, countRows(t, db, "identities"))
	assert.Equal(t, 1, countRows(t, db, "api_keys"))
	assert.Equal(t, 1, countRows(t, db, "groups"))
	assert.Equal(t, 1, countAdministratorsPolicies(t, db))
	assert.Len(t, groupMembers(t, db, administratorsGroup(t, db)), 1)
}

func TestBootstrap_IdentitiesExist(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	identities := identityservice.NewIdentityService(identityrepo.NewPostgresIdentityRepository(db), authz.AllowAll())
	_, err := identities.Create(t.Context(), audit.SystemActor(), identityservice.IdentityInput{
		Username: "ada", DisplayName: "Ada Lovelace",
	})
	require.NoError(t, err)

	key, err := Bootstrap(t.Context(), db, testPepper)
	require.NoError(t, err)

	assert.Empty(t, key)
	assert.Equal(t, 1, countRows(t, db, "identities"), "no admin may be created")
	assert.Equal(t, 0, countRows(t, db, "api_keys"))
	assert.Equal(t, 0, countRows(t, db, "groups"))
	assert.Equal(t, 0, countAdministratorsPolicies(t, db))
}

func TestBootstrap_FailureLeavesNothingBehind(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	// Creating the API key fails, after the admin was created.
	_, err := db.ExecContext(t.Context(), "DROP TABLE api_keys")
	require.NoError(t, err)

	key, err := Bootstrap(t.Context(), db, testPepper)

	require.Error(t, err)
	assert.Empty(t, key)
	assert.Equal(t, 0, countRows(t, db, "identities"), "the admin must not survive without its key")
	assert.Equal(t, 0, countRows(t, db, "groups"))
	assert.Equal(t, 0, countAdministratorsPolicies(t, db))
}

// TestBootstrap_GroupNameTaken checks that a group named administrators, which is left over when every
// identity was deleted after the first start, fails the start without leaving anything behind.
func TestBootstrap_GroupNameTaken(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO groups (id, name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, 'administrators', $1, $1, now(), now())`,
		uuid.NewV4(),
	)
	require.NoError(t, err)

	key, err := Bootstrap(t.Context(), db, testPepper)

	require.ErrorIs(t, err, groupservice.ErrGroupNameTaken)
	assert.Empty(t, key)
	assert.Equal(t, 0, countRows(t, db, "identities"))
	assert.Equal(t, 1, countRows(t, db, "groups"))
	assert.Equal(t, 0, countAdministratorsPolicies(t, db))
}

// TestBootstrap_PolicyNameTaken checks that a policy named administrators, which is left over when every
// identity was deleted after the first start, fails the start without leaving anything behind.
func TestBootstrap_PolicyNameTaken(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO policies (id, name, source, created_by, updated_by, created_at, updated_at)
		VALUES ($1, 'administrators', 'permit (principal, action, resource);', $1, $1, now(), now())`,
		uuid.NewV4(),
	)
	require.NoError(t, err)

	key, err := Bootstrap(t.Context(), db, testPepper)

	require.ErrorIs(t, err, policyservice.ErrPolicyNameTaken)
	assert.Empty(t, key)
	assert.Equal(t, 0, countRows(t, db, "identities"))
	assert.Equal(t, 0, countRows(t, db, "groups"))
	assert.Equal(t, 1, countAdministratorsPolicies(t, db))
}

// TestBootstrap_ConcurrentStarts checks what instances that start at once end up with, whichever wins
// and however they interleave: one key that was returned to one of them, and one admin.
func TestBootstrap_ConcurrentStarts(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	keys := make([]string, 2)

	var wg sync.WaitGroup
	for i := range keys {
		wg.Go(func() {
			// A loser either fails on the username or, if it starts late enough, finds the admin.
			keys[i], _ = Bootstrap(t.Context(), db, testPepper)
		})
	}
	wg.Wait()

	assert.NotEqual(t, keys[0] == "", keys[1] == "", "exactly one instance must get a key: %q", keys)
	assert.Equal(t, 1, countRows(t, db, "identities"))
	assert.Equal(t, 1, countRows(t, db, "api_keys"))
	assert.Equal(t, 1, countRows(t, db, "groups"))
	assert.Equal(t, 1, countAdministratorsPolicies(t, db))
	assert.Len(t, groupMembers(t, db, administratorsGroup(t, db)), 1)
}
