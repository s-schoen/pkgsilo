package app

import (
	"sync"
	"testing"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/audit"
	"pkgsilo/core/authz"
	apikeyrepo "pkgsilo/feature/apikey/repository"
	apikeyservice "pkgsilo/feature/apikey/service"
	groupservice "pkgsilo/feature/group/service"
	identityrepo "pkgsilo/feature/identity/repository"
	identityservice "pkgsilo/feature/identity/service"
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
	assert.Equal(t, "admin", admin.Username)
	assert.Equal(t, "Administrator", admin.DisplayName)
	assert.Equal(t, audit.SystemActor(), admin.CreatedBy)
	assert.Equal(t, []uuid.UUID{admin.ID}, groupMembers(t, db, administratorsGroupID))

	apiKeys := apikeyservice.NewAPIKeyService(apikeyrepo.NewPostgresAPIKeyRepository(db), authz.AllowAll(), testPepper)
	apiKey, err := apiKeys.Authenticate(t.Context(), key)
	require.NoError(t, err, "the returned key must authenticate")
	assert.Equal(t, admin.ID, apiKey.IdentityID)
	assert.Equal(t, "bootstrap", apiKey.Name)
	assert.Nil(t, apiKey.ExpiresAt, "the first key must never expire")
	assert.Equal(t, audit.SystemActor(), apiKey.CreatedBy)
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
	assert.Len(t, groupMembers(t, db, administratorsGroupID), 1)
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
	assert.Empty(t, groupMembers(t, db, administratorsGroupID))
}

func TestBootstrap_AdministratorsGroupDeleted(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	_, err := db.ExecContext(t.Context(), "DELETE FROM groups WHERE id = $1", administratorsGroupID)
	require.NoError(t, err)

	key, err := Bootstrap(t.Context(), db, testPepper)

	require.ErrorIs(t, err, groupservice.ErrGroupNotFound)
	assert.Empty(t, key)
	assert.Equal(t, 0, countRows(t, db, "identities"), "an admin without the group could do nothing")
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
	assert.Len(t, groupMembers(t, db, administratorsGroupID), 1)
}
