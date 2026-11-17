package repository

import (
	"testing"
	"time"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/test/databasetest"
)

var baseTime = time.Date(2026, time.November, 17, 12, 0, 0, 123456000, time.UTC)

// newTestRepo returns a repository on a new, empty database, and the database for setting up the
// identities that records refer to.
func newTestRepo(t *testing.T) (*PostgresPasswordRepository, *sqlx.DB) {
	t.Helper()

	db := databasetest.New(t)

	return NewPostgresPasswordRepository(db), db
}

// insertIdentity stores an identity row with the given username for records to refer to and returns its ID.
// The identities table belongs to the identity feature, so the row is written directly.
func insertIdentity(t *testing.T, db *sqlx.DB, username string) uuid.UUID {
	t.Helper()

	id := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO identities (id, username, display_name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, 'Owner', $1, $1, $3, $3)`,
		id, username, baseTime,
	)
	require.NoError(t, err)

	return id
}

// newRecord returns a record of the identity's password with the given hash, set at baseTime. Times have
// microsecond precision, like PostgreSQL timestamps, so records survive a round trip unchanged.
func newRecord(identityID uuid.UUID, hash string) PasswordRecord {
	return PasswordRecord{IdentityID: identityID, Hash: hash, SetAt: baseTime}
}

// setRecord stores a password of the identity with the given hash and returns its record.
func setRecord(t *testing.T, repo *PostgresPasswordRepository, identityID uuid.UUID, hash string) PasswordRecord {
	t.Helper()

	rec, err := repo.Set(t.Context(), newRecord(identityID, hash))
	require.NoError(t, err)

	return rec
}

func TestPostgresPasswordRepository_SetAndGet(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	rec := newRecord(insertIdentity(t, db, "ada"), "$argon2id$first")

	stored, err := repo.Set(t.Context(), rec)
	require.NoError(t, err)
	assert.Equal(t, rec, stored)

	got, err := repo.Get(t.Context(), rec.IdentityID)
	require.NoError(t, err)
	assert.Equal(t, rec, got)
}

// TestPostgresPasswordRepository_Set_Replaces checks that setting a password of an identity that has one
// replaces it, since an identity has at most one.
func TestPostgresPasswordRepository_Set_Replaces(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db, "ada")
	setRecord(t, repo, identityID, "$argon2id$first")
	replacement := PasswordRecord{IdentityID: identityID, Hash: "$argon2id$second", SetAt: baseTime.Add(time.Hour)}

	stored, err := repo.Set(t.Context(), replacement)
	require.NoError(t, err)
	assert.Equal(t, replacement, stored)

	got, err := repo.Get(t.Context(), identityID)
	require.NoError(t, err)
	assert.Equal(t, replacement, got)
	var count int
	require.NoError(t, db.GetContext(t.Context(), &count, `SELECT count(*) FROM passwords`))
	assert.Equal(t, 1, count)
}

func TestPostgresPasswordRepository_Set_IdentityNotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	_, err := repo.Set(t.Context(), newRecord(uuid.NewV4(), "$argon2id$first"))

	assert.ErrorIs(t, err, ErrIdentityNotFound)
}

func TestPostgresPasswordRepository_Get_NotFound(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	setRecord(t, repo, insertIdentity(t, db, "grace"), "$argon2id$grace")

	tests := []struct {
		name       string
		identityID uuid.UUID
	}{
		{name: "identity without a password", identityID: insertIdentity(t, db, "ada")},
		{name: "unknown identity", identityID: uuid.NewV4()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := repo.Get(t.Context(), tt.identityID)
			assert.ErrorIs(t, err, ErrNotFound)
		})
	}
}

func TestPostgresPasswordRepository_UpdateHash(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db, "ada")
	rec := setRecord(t, repo, identityID, "$argon2id$old")

	updated, err := repo.UpdateHash(t.Context(), identityID, "$argon2id$old", "$argon2id$new")
	require.NoError(t, err)
	assert.True(t, updated)

	got, err := repo.Get(t.Context(), identityID)
	require.NoError(t, err)
	assert.Equal(t, PasswordRecord{IdentityID: identityID, Hash: "$argon2id$new", SetAt: rec.SetAt}, got,
		"a rehash must not change when the password was set")
}

// TestPostgresPasswordRepository_UpdateHash_Changed checks that a rehash changes nothing once the hash it
// read is gone, so that it can't undo a change or a delete.
func TestPostgresPasswordRepository_UpdateHash_Changed(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	changed := insertIdentity(t, db, "ada")
	setRecord(t, repo, changed, "$argon2id$changed")

	tests := []struct {
		name       string
		identityID uuid.UUID
	}{
		{name: "changed since", identityID: changed},
		{name: "deleted since", identityID: insertIdentity(t, db, "grace")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			updated, err := repo.UpdateHash(t.Context(), tt.identityID, "$argon2id$old", "$argon2id$new")
			require.NoError(t, err)
			assert.False(t, updated)
		})
	}
	got, err := repo.Get(t.Context(), changed)
	require.NoError(t, err)
	assert.Equal(t, "$argon2id$changed", got.Hash)
}

func TestPostgresPasswordRepository_Delete(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID, otherID := insertIdentity(t, db, "ada"), insertIdentity(t, db, "grace")
	setRecord(t, repo, identityID, "$argon2id$ada")
	setRecord(t, repo, otherID, "$argon2id$grace")

	require.NoError(t, repo.Delete(t.Context(), identityID))

	_, err := repo.Get(t.Context(), identityID)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = repo.Get(t.Context(), otherID)
	assert.NoError(t, err, "the passwords of other identities stay")
}

func TestPostgresPasswordRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)

	tests := []struct {
		name       string
		identityID uuid.UUID
	}{
		{name: "identity without a password", identityID: insertIdentity(t, db, "ada")},
		{name: "unknown identity", identityID: uuid.NewV4()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, repo.Delete(t.Context(), tt.identityID), ErrNotFound)
		})
	}
}

func TestPostgresPasswordRepository_GetCredentials(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	ada, grace := insertIdentity(t, db, "ada"), insertIdentity(t, db, "grace")
	setRecord(t, repo, ada, "$argon2id$ada")
	hash := "$argon2id$ada"

	tests := []struct {
		name     string
		username string
		want     CredentialsRecord
	}{
		{name: "with a password", username: "ada", want: CredentialsRecord{IdentityID: ada, Hash: &hash}},
		{name: "without a password", username: "grace", want: CredentialsRecord{IdentityID: grace}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := repo.GetCredentials(t.Context(), tt.username)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPostgresPasswordRepository_GetCredentials_IdentityNotFound(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	insertIdentity(t, db, "ada")

	tests := []string{"grace", "Ada", "ada ", ""}
	for _, username := range tests {
		t.Run(username, func(t *testing.T) {
			t.Parallel()
			_, err := repo.GetCredentials(t.Context(), username)
			assert.ErrorIs(t, err, ErrIdentityNotFound)
		})
	}
}

// insertGroup stores a group row with the identities as members and returns its ID. The tables belong to
// the group feature, so the rows are written directly.
func insertGroup(t *testing.T, db *sqlx.DB, members ...uuid.UUID) uuid.UUID {
	t.Helper()

	id := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO groups (id, name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $1, $1, $3, $3)`,
		id, "group-"+id.String(), baseTime,
	)
	require.NoError(t, err)
	for _, member := range members {
		_, err := db.ExecContext(t.Context(), `
			INSERT INTO group_members (group_id, identity_id, created_by, created_at) VALUES ($1, $2, $2, $3)`,
			id, member, baseTime,
		)
		require.NoError(t, err)
	}

	return id
}

func TestPostgresPasswordRepository_IdentityGroups(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	ada, grace := insertIdentity(t, db, "ada"), insertIdentity(t, db, "grace")
	maintainers, reviewers := insertGroup(t, db, ada), insertGroup(t, db, ada, grace)
	// Another identity's group, which must not be listed.
	insertGroup(t, db, grace)

	got, err := repo.IdentityGroups(t.Context(), ada)
	require.NoError(t, err)

	assert.ElementsMatch(t, []uuid.UUID{maintainers, reviewers}, got)
}

func TestPostgresPasswordRepository_IdentityGroups_None(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	insertGroup(t, db)

	got, err := repo.IdentityGroups(t.Context(), insertIdentity(t, db, "ada"))
	require.NoError(t, err)

	assert.Empty(t, got)
}

func TestPostgresPasswordRepository_IdentityGroups_IdentityNotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	_, err := repo.IdentityGroups(t.Context(), uuid.NewV4())

	assert.ErrorIs(t, err, ErrIdentityNotFound)
}

func TestPostgresPasswordRepository_DeletingIdentityDeletesItsPassword(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db, "ada")
	setRecord(t, repo, identityID, "$argon2id$ada")

	_, err := db.ExecContext(t.Context(), `DELETE FROM identities WHERE id = $1`, identityID)
	require.NoError(t, err)

	_, err = repo.Get(t.Context(), identityID)
	assert.ErrorIs(t, err, ErrNotFound)
}
