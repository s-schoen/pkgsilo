package repository

import (
	"testing"
	"time"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/pagination"
	"pkgsilo/test/databasetest"
)

var baseTime = time.Date(2026, time.November, 5, 12, 0, 0, 123456000, time.UTC)

// newTestRepo returns a repository on a new, empty database, and the database for setting up the
// identities that records refer to.
func newTestRepo(t *testing.T) (*PostgresAPIKeyRepository, *sqlx.DB) {
	t.Helper()

	db := databasetest.New(t)

	return NewPostgresAPIKeyRepository(db), db
}

// insertIdentity stores an identity row for records to refer to and returns its ID. The identities
// table belongs to the identity feature, so the row is written directly.
func insertIdentity(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()

	id := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO identities (id, username, display_name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, 'Owner', $1, $1, $3, $3)`,
		id, "owner-"+id.String(), baseTime,
	)
	require.NoError(t, err)

	return id
}

// newRecord returns a record of the identity created at the given time, with a unique hash. Times
// have microsecond precision, like PostgreSQL timestamps, so records survive a round trip unchanged.
func newRecord(identityID uuid.UUID, createdAt time.Time) APIKeyRecord {
	id := uuid.NewV4()

	return APIKeyRecord{
		ID:         id,
		IdentityID: identityID,
		Name:       "ci",
		KeyHash:    id[:],
		KeySuffix:  "WXYZ",
		CreatedBy:  uuid.NewV4(),
		CreatedAt:  createdAt,
	}
}

// insertRecord stores a new record of the identity created at baseTime and returns it.
func insertRecord(t *testing.T, repo *PostgresAPIKeyRepository, identityID uuid.UUID) APIKeyRecord {
	t.Helper()

	rec, err := repo.Insert(t.Context(), newRecord(identityID, baseTime))
	require.NoError(t, err)

	return rec
}

func TestPostgresAPIKeyRepository_InsertAndGet(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	expiresAt, lastUsedAt := baseTime.Add(time.Hour), baseTime.Add(time.Minute)
	withTimes := newRecord(insertIdentity(t, db), baseTime)
	withTimes.ExpiresAt, withTimes.LastUsedAt = &expiresAt, &lastUsedAt

	tests := []struct {
		name string
		rec  APIKeyRecord
	}{
		{name: "without optional times", rec: newRecord(insertIdentity(t, db), baseTime)},
		{name: "with optional times", rec: withTimes},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stored, err := repo.Insert(t.Context(), tt.rec)
			require.NoError(t, err)
			assert.Equal(t, tt.rec, stored)

			got, err := repo.Get(t.Context(), tt.rec.IdentityID, tt.rec.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.rec, got)
		})
	}
}

func TestPostgresAPIKeyRepository_Insert_IdentityNotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	_, err := repo.Insert(t.Context(), newRecord(uuid.NewV4(), baseTime))

	assert.ErrorIs(t, err, ErrIdentityNotFound)
}

func TestPostgresAPIKeyRepository_Insert_DuplicateHashFails(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db)
	rec := insertRecord(t, repo, identityID)
	duplicate := newRecord(identityID, baseTime)
	duplicate.KeyHash = rec.KeyHash

	_, err := repo.Insert(t.Context(), duplicate)

	assert.Error(t, err)
}

func TestPostgresAPIKeyRepository_Get_NotFound(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID, otherID := insertIdentity(t, db), insertIdentity(t, db)
	rec := insertRecord(t, repo, identityID)

	tests := []struct {
		name       string
		identityID uuid.UUID
		id         uuid.UUID
	}{
		{name: "unknown id", identityID: identityID, id: uuid.NewV4()},
		{name: "key of another identity", identityID: otherID, id: rec.ID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := repo.Get(t.Context(), tt.identityID, tt.id)

			assert.ErrorIs(t, err, ErrNotFound)
		})
	}
}

func TestPostgresAPIKeyRepository_GetByHash(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	rec := insertRecord(t, repo, insertIdentity(t, db))

	got, err := repo.GetByHash(t.Context(), rec.KeyHash)
	require.NoError(t, err)

	assert.Equal(t, rec, got)
}

func TestPostgresAPIKeyRepository_GetByHash_NotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	_, err := repo.GetByHash(t.Context(), []byte("unknown"))

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresAPIKeyRepository_ListAll(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db)
	first := newRecord(identityID, baseTime)
	second := newRecord(identityID, baseTime.Add(time.Minute))
	third := newRecord(identityID, baseTime.Add(2*time.Minute))
	// Inserted out of order, so the result order can only come from the creation time. The key of
	// another identity must not show up.
	for _, rec := range []APIKeyRecord{second, third, first, newRecord(insertIdentity(t, db), baseTime)} {
		_, err := repo.Insert(t.Context(), rec)
		require.NoError(t, err)
	}

	got, err := repo.ListAll(t.Context(), identityID)
	require.NoError(t, err)

	assert.Equal(t, []APIKeyRecord{first, second, third}, got)
}

func TestPostgresAPIKeyRepository_ListAll_None(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)

	for name, id := range map[string]uuid.UUID{"no keys": insertIdentity(t, db), "missing": uuid.NewV4()} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := repo.ListAll(t.Context(), id)
			require.NoError(t, err)

			assert.Empty(t, got)
		})
	}
}

func TestPostgresAPIKeyRepository_ListAll_Ceiling(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db)

	insertKeys := func(count int) {
		_, err := db.ExecContext(t.Context(), `
			INSERT INTO api_keys (id, identity_id, name, key_hash, key_suffix, created_by, created_at)
			SELECT gen_random_uuid(), $1, 'ci', gen_random_uuid()::text::bytea, 'WXYZ', $1, $2
			FROM generate_series(1, $3::int)`,
			identityID, baseTime, count,
		)
		require.NoError(t, err)
	}

	insertKeys(pagination.MaxAllRows)
	got, err := repo.ListAll(t.Context(), identityID)
	require.NoError(t, err)
	assert.Len(t, got, pagination.MaxAllRows)

	insertKeys(1)
	got, err = repo.ListAll(t.Context(), identityID)
	require.ErrorIs(t, err, ErrTooManyRows)
	assert.Nil(t, got)
}

func TestPostgresAPIKeyRepository_UpdateLastUsed(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	rec := insertRecord(t, repo, insertIdentity(t, db))
	lastUsedAt := baseTime.Add(time.Hour)

	require.NoError(t, repo.UpdateLastUsed(t.Context(), rec.ID, lastUsedAt))

	got, err := repo.Get(t.Context(), rec.IdentityID, rec.ID)
	require.NoError(t, err)
	rec.LastUsedAt = &lastUsedAt
	assert.Equal(t, rec, got, "only the last use changes")
}

func TestPostgresAPIKeyRepository_UpdateLastUsed_MissingRecord(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	assert.NoError(t, repo.UpdateLastUsed(t.Context(), uuid.NewV4(), baseTime))
}

func TestPostgresAPIKeyRepository_Delete(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db)
	rec, kept := insertRecord(t, repo, identityID), insertRecord(t, repo, identityID)

	require.NoError(t, repo.Delete(t.Context(), identityID, rec.ID))

	_, err := repo.Get(t.Context(), identityID, rec.ID)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = repo.Get(t.Context(), identityID, kept.ID)
	assert.NoError(t, err, "other keys of the identity stay")
}

func TestPostgresAPIKeyRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID, otherID := insertIdentity(t, db), insertIdentity(t, db)
	rec := insertRecord(t, repo, identityID)

	tests := []struct {
		name       string
		identityID uuid.UUID
		id         uuid.UUID
	}{
		{name: "unknown id", identityID: identityID, id: uuid.NewV4()},
		{name: "key of another identity", identityID: otherID, id: rec.ID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, repo.Delete(t.Context(), tt.identityID, tt.id), ErrNotFound)
		})
	}
	_, err := repo.Get(t.Context(), identityID, rec.ID)
	assert.NoError(t, err, "the key must survive a delete through another identity")
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

func TestPostgresAPIKeyRepository_IdentityGroups(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	ada, grace := insertIdentity(t, db), insertIdentity(t, db)
	maintainers, reviewers := insertGroup(t, db, ada), insertGroup(t, db, ada, grace)
	// Another identity's group, which must not be listed.
	insertGroup(t, db, grace)

	got, err := repo.IdentityGroups(t.Context(), ada)
	require.NoError(t, err)

	assert.ElementsMatch(t, []uuid.UUID{maintainers, reviewers}, got)
}

func TestPostgresAPIKeyRepository_IdentityGroups_None(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	insertGroup(t, db)

	got, err := repo.IdentityGroups(t.Context(), insertIdentity(t, db))
	require.NoError(t, err)

	assert.Empty(t, got)
}

func TestPostgresAPIKeyRepository_IdentityGroups_IdentityNotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	_, err := repo.IdentityGroups(t.Context(), uuid.NewV4())

	assert.ErrorIs(t, err, ErrIdentityNotFound)
}

func TestPostgresAPIKeyRepository_DeletingIdentityDeletesItsKeys(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db)
	rec := insertRecord(t, repo, identityID)

	_, err := db.ExecContext(t.Context(), `DELETE FROM identities WHERE id = $1`, identityID)
	require.NoError(t, err)

	_, err = repo.GetByHash(t.Context(), rec.KeyHash)
	assert.ErrorIs(t, err, ErrNotFound)
}
