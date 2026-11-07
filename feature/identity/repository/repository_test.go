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

// newTestRepo returns a repository on a new, empty database.
func newTestRepo(t *testing.T) *PostgresIdentityRepository {
	t.Helper()

	return NewPostgresIdentityRepository(databasetest.New(t))
}

// newRecord returns a record created at the given time. Times have microsecond precision,
// like PostgreSQL timestamps, so records survive a round trip unchanged.
func newRecord(username string, createdAt time.Time) IdentityRecord {
	actor := uuid.NewV4()

	return IdentityRecord{
		ID:          uuid.NewV4(),
		Username:    username,
		DisplayName: "Display " + username,
		CreatedBy:   actor,
		UpdatedBy:   actor,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
}

var baseTime = time.Date(2026, time.October, 24, 12, 0, 0, 123456000, time.UTC)

func TestPostgresIdentityRepository_InsertAndGet(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	rec := newRecord("ada", baseTime)

	stored, err := repo.Insert(t.Context(), rec)
	require.NoError(t, err)
	assert.Equal(t, rec, stored)

	got, err := repo.Get(t.Context(), rec.ID)
	require.NoError(t, err)
	assert.Equal(t, rec, got)
}

func TestPostgresIdentityRepository_Insert_DuplicateUsername(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_, err := repo.Insert(t.Context(), newRecord("ada", baseTime))
	require.NoError(t, err)

	_, err = repo.Insert(t.Context(), newRecord("ada", baseTime))

	assert.ErrorIs(t, err, ErrDuplicateUsername)
}

func TestPostgresIdentityRepository_Get_NotFound(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)

	_, err := repo.Get(t.Context(), uuid.NewV4())

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresIdentityRepository_ListAll(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	first := newRecord("first", baseTime)
	second := newRecord("second", baseTime.Add(time.Minute))
	third := newRecord("third", baseTime.Add(2*time.Minute))
	// Inserted out of order, so the result order can only come from the creation time.
	for _, rec := range []IdentityRecord{second, third, first} {
		_, err := repo.Insert(t.Context(), rec)
		require.NoError(t, err)
	}

	got, err := repo.ListAll(t.Context())
	require.NoError(t, err)

	assert.Equal(t, []IdentityRecord{first, second, third}, got)
}

func TestPostgresIdentityRepository_ListAll_Empty(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)

	got, err := repo.ListAll(t.Context())
	require.NoError(t, err)

	assert.Empty(t, got)
}

// insertIdentities stores count identity rows, in one statement to keep the test fast.
func insertIdentities(t *testing.T, db *sqlx.DB, prefix string, count int) {
	t.Helper()

	_, err := db.ExecContext(t.Context(), `
		INSERT INTO identities (id, username, display_name, created_by, updated_by, created_at, updated_at)
		SELECT gen_random_uuid(), $1 || n, 'Test', $2::uuid, $2::uuid, $3::timestamptz, $3::timestamptz
		FROM generate_series(1, $4::int) AS n`,
		prefix, uuid.NewV4(), baseTime, count,
	)
	require.NoError(t, err)
}

func TestPostgresIdentityRepository_ListAll_Ceiling(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	repo := NewPostgresIdentityRepository(db)

	insertIdentities(t, db, "at-ceiling-", pagination.MaxAllRows)
	got, err := repo.ListAll(t.Context())
	require.NoError(t, err)
	assert.Len(t, got, pagination.MaxAllRows)

	insertIdentities(t, db, "over-ceiling-", 1)
	got, err = repo.ListAll(t.Context())
	require.ErrorIs(t, err, ErrTooManyRows)
	assert.Nil(t, got)
}

func TestPostgresIdentityRepository_ListMemberships(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	repo := NewPostgresIdentityRepository(db)
	ada := newRecord("ada", baseTime)
	grace := newRecord("grace", baseTime)
	margaret := newRecord("margaret", baseTime)
	for _, rec := range []IdentityRecord{ada, grace, margaret} {
		_, err := repo.Insert(t.Context(), rec)
		require.NoError(t, err)
	}
	maintainers := insertGroup(t, db, "maintainers", ada.ID, grace.ID)
	reviewers := insertGroup(t, db, "reviewers", ada.ID, margaret.ID)

	got, err := repo.ListMemberships(t.Context(), []uuid.UUID{ada.ID, grace.ID})
	require.NoError(t, err)

	assert.ElementsMatch(t, []MembershipRecord{
		{IdentityID: ada.ID, GroupID: maintainers},
		{IdentityID: ada.ID, GroupID: reviewers},
		{IdentityID: grace.ID, GroupID: maintainers},
	}, got)
}

func TestPostgresIdentityRepository_ListMemberships_None(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)

	tests := []struct {
		name string
		ids  []uuid.UUID
	}{
		{name: "no IDs", ids: []uuid.UUID{}},
		{name: "nil", ids: nil},
		{name: "not a member", ids: []uuid.UUID{uuid.NewV4()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := repo.ListMemberships(t.Context(), tt.ids)
			require.NoError(t, err)

			assert.Empty(t, got)
		})
	}
}

// insertGroup stores a group with the identities as members and returns its ID. The tables belong to the
// group feature, so the rows are written directly.
func insertGroup(t *testing.T, db *sqlx.DB, name string, members ...uuid.UUID) uuid.UUID {
	t.Helper()

	id := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO groups (id, name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $1, $1, $3, $3)`,
		id, name, baseTime,
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

func TestPostgresIdentityRepository_Update(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	rec := newRecord("ada", baseTime)
	_, err := repo.Insert(t.Context(), rec)
	require.NoError(t, err)

	update := IdentityRecord{
		ID:          rec.ID,
		Username:    "ada-lovelace",
		DisplayName: "Ada Lovelace",
		UpdatedBy:   uuid.NewV4(),
		UpdatedAt:   baseTime.Add(time.Hour),
		// Immutable: must not overwrite the stored values.
		CreatedBy: uuid.NewV4(),
		CreatedAt: baseTime.Add(time.Hour),
	}
	want := update
	want.CreatedBy, want.CreatedAt = rec.CreatedBy, rec.CreatedAt

	stored, err := repo.Update(t.Context(), update)
	require.NoError(t, err)
	assert.Equal(t, want, stored)

	got, err := repo.Get(t.Context(), rec.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestPostgresIdentityRepository_Update_NotFound(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)

	_, err := repo.Update(t.Context(), newRecord("ada", baseTime))

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresIdentityRepository_Update_DuplicateUsername(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	_, err := repo.Insert(t.Context(), newRecord("ada", baseTime))
	require.NoError(t, err)
	grace := newRecord("grace", baseTime)
	_, err = repo.Insert(t.Context(), grace)
	require.NoError(t, err)

	grace.Username = "ada"
	_, err = repo.Update(t.Context(), grace)

	assert.ErrorIs(t, err, ErrDuplicateUsername)
}

func TestPostgresIdentityRepository_Delete(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)
	rec := newRecord("ada", baseTime)
	_, err := repo.Insert(t.Context(), rec)
	require.NoError(t, err)

	require.NoError(t, repo.Delete(t.Context(), rec.ID))

	_, err = repo.Get(t.Context(), rec.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresIdentityRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	repo := newTestRepo(t)

	err := repo.Delete(t.Context(), uuid.NewV4())

	assert.ErrorIs(t, err, ErrNotFound)
}
