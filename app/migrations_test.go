package app

import (
	"testing"
	"time"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/audit"
	"pkgsilo/core/authz"
	"pkgsilo/test/databasetest"
)

const (
	createPoliciesUp   = "20261106190643_create_policies.up.sql"
	createPoliciesDown = "20261106190643_create_policies.down.sql"
)

// insertIdentityRow stores an identity created by the given actor, as a database from before the
// migration holds it, and returns its ID.
func insertIdentityRow(t *testing.T, db *sqlx.DB, username string, createdBy uuid.UUID) uuid.UUID {
	t.Helper()

	id := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO identities (id, username, display_name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, 'Test', $3, $3, $4, $4)`,
		id, username, createdBy, time.Now(),
	)
	require.NoError(t, err)

	return id
}

// assertCanRead asserts whether the actor may read the identity, by the seeded policies.
func assertCanRead(t *testing.T, want bool, db *sqlx.DB, actor, identity uuid.UUID) {
	t.Helper()

	decider, err := authz.New(db).For(t.Context(), actor)
	require.NoError(t, err)

	can, err := decider.Can(
		authz.NewAction("identity:read", authz.ActionGroupRead),
		authz.Resource{Type: authz.IdentityType, ID: identity},
	)
	require.NoError(t, err)
	assert.Equal(t, want, can)
}

func TestCreatePoliciesMigration_ExistingAdmin(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	databasetest.RunMigration(t, db, createPoliciesDown)
	admin := insertIdentityRow(t, db, "admin", audit.SystemActor())
	ada := insertIdentityRow(t, db, "ada", admin)

	databasetest.RunMigration(t, db, createPoliciesUp)

	assert.Equal(t, []uuid.UUID{admin}, groupMembers(t, db, administratorsGroupID))
	assertCanRead(t, true, db, admin, ada)
	assertCanRead(t, true, db, ada, ada)
	assertCanRead(t, false, db, ada, admin)
}

// TestCreatePoliciesMigration_RenamedAdmin checks that the first start's identity keeps its access under
// another name, and that an unrelated identity that has since taken the name gets none.
func TestCreatePoliciesMigration_RenamedAdmin(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	databasetest.RunMigration(t, db, createPoliciesDown)
	root := insertIdentityRow(t, db, "root", audit.SystemActor())
	impostor := insertIdentityRow(t, db, "admin", root)

	databasetest.RunMigration(t, db, createPoliciesUp)

	assert.Equal(t, []uuid.UUID{root}, groupMembers(t, db, administratorsGroupID))
	assertCanRead(t, true, db, root, impostor)
	assertCanRead(t, false, db, impostor, root)
}

// TestCreatePoliciesMigration_DeletedAdmin checks that no one inherits the access of a deleted first
// identity: the accepted lockout, which an operator recovers from in the database.
func TestCreatePoliciesMigration_DeletedAdmin(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	databasetest.RunMigration(t, db, createPoliciesDown)
	insertIdentityRow(t, db, "ada", uuid.NewV4())

	databasetest.RunMigration(t, db, createPoliciesUp)

	assert.Empty(t, groupMembers(t, db, administratorsGroupID))
}

// TestCreatePoliciesMigration_ExistingGroup checks that a group that already has the seeded group's name
// stays as it is and gives its members no access, while the seeded group takes another name.
func TestCreatePoliciesMigration_ExistingGroup(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	databasetest.RunMigration(t, db, createPoliciesDown)
	admin := insertIdentityRow(t, db, "admin", audit.SystemActor())
	ada := insertIdentityRow(t, db, "ada", admin)
	existing := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO groups (id, name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, 'administrators', $2, $2, $3, $3)`,
		existing, admin, time.Now(),
	)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `
		INSERT INTO group_members (group_id, identity_id, created_by, created_at) VALUES ($1, $2, $3, $4)`,
		existing, ada, admin, time.Now(),
	)
	require.NoError(t, err)

	databasetest.RunMigration(t, db, createPoliciesUp)

	var existingName, seededName string
	require.NoError(t, db.GetContext(t.Context(), &existingName, "SELECT name FROM groups WHERE id = $1", existing))
	require.NoError(t, db.GetContext(t.Context(), &seededName, "SELECT name FROM groups WHERE id = $1",
		administratorsGroupID))
	assert.Equal(t, "administrators", existingName)
	assert.Equal(t, "administrators-"+administratorsGroupID.String(), seededName)

	assert.Equal(t, []uuid.UUID{ada}, groupMembers(t, db, existing))
	assert.Equal(t, []uuid.UUID{admin}, groupMembers(t, db, administratorsGroupID))
	assertCanRead(t, true, db, admin, ada)
	assertCanRead(t, false, db, ada, admin)
}
