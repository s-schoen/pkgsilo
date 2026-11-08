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

// newTestRepo returns a repository on a new database, which holds no groups, and the database for setting
// up the identities that member records refer to.
func newTestRepo(t *testing.T) (*PostgresGroupRepository, *sqlx.DB) {
	t.Helper()

	db := databasetest.New(t)

	return NewPostgresGroupRepository(db), db
}

// insertIdentity stores an identity row for member records to refer to and returns its ID. The
// identities table belongs to the identity feature, so the row is written directly.
func insertIdentity(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()

	id := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO identities (id, username, display_name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, 'Member', $1, $1, $3, $3)`,
		id, "member-"+id.String(), baseTime,
	)
	require.NoError(t, err)

	return id
}

// newRecord returns a record created at the given time. Times have microsecond precision,
// like PostgreSQL timestamps, so records survive a round trip unchanged.
func newRecord(name string, createdAt time.Time) GroupRecord {
	actor := uuid.NewV4()

	return GroupRecord{
		ID:        uuid.NewV4(),
		Name:      name,
		CreatedBy: actor,
		UpdatedBy: actor,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

// newMember returns a member record of the identity in the group, created at the given time.
func newMember(groupID, identityID uuid.UUID, createdAt time.Time) MemberRecord {
	return MemberRecord{GroupID: groupID, IdentityID: identityID, CreatedBy: uuid.NewV4(), CreatedAt: createdAt}
}

// insertGroup stores a new group record created at baseTime and returns it.
func insertGroup(t *testing.T, repo *PostgresGroupRepository, name string) GroupRecord {
	t.Helper()

	rec, err := repo.Insert(t.Context(), newRecord(name, baseTime))
	require.NoError(t, err)

	return rec
}

var baseTime = time.Date(2026, time.November, 2, 12, 0, 0, 123456000, time.UTC)

func TestPostgresGroupRepository_InsertAndGet(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	rec := newRecord("Maintainers", baseTime)

	stored, err := repo.Insert(t.Context(), rec)
	require.NoError(t, err)
	assert.Equal(t, rec, stored)

	got, err := repo.Get(t.Context(), rec.ID)
	require.NoError(t, err)
	assert.Equal(t, rec, got)
}

func TestPostgresGroupRepository_Insert_DuplicateName(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	insertGroup(t, repo, "Maintainers")

	_, err := repo.Insert(t.Context(), newRecord("Maintainers", baseTime))

	assert.ErrorIs(t, err, ErrDuplicateName)
}

func TestPostgresGroupRepository_Get_NotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	_, err := repo.Get(t.Context(), uuid.NewV4())

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresGroupRepository_ListAll(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	first := newRecord("first", baseTime)
	second := newRecord("second", baseTime.Add(time.Minute))
	third := newRecord("third", baseTime.Add(2*time.Minute))
	// Inserted out of order, so the result order can only come from the creation time.
	for _, rec := range []GroupRecord{second, third, first} {
		_, err := repo.Insert(t.Context(), rec)
		require.NoError(t, err)
	}

	got, err := repo.ListAll(t.Context())
	require.NoError(t, err)

	assert.Equal(t, []GroupRecord{first, second, third}, got)
}

func TestPostgresGroupRepository_ListAll_Empty(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	got, err := repo.ListAll(t.Context())
	require.NoError(t, err)

	assert.Empty(t, got)
}

// insertGroups stores count group rows, in one statement to keep the test fast.
func insertGroups(t *testing.T, db *sqlx.DB, prefix string, count int) {
	t.Helper()

	_, err := db.ExecContext(t.Context(), `
		INSERT INTO groups (id, name, created_by, updated_by, created_at, updated_at)
		SELECT gen_random_uuid(), $1 || n, $2::uuid, $2::uuid, $3::timestamptz, $3::timestamptz
		FROM generate_series(1, $4::int) AS n`,
		prefix, uuid.NewV4(), baseTime, count,
	)
	require.NoError(t, err)
}

func TestPostgresGroupRepository_ListAll_Ceiling(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)

	insertGroups(t, db, "at-ceiling-", pagination.MaxAllRows)
	got, err := repo.ListAll(t.Context())
	require.NoError(t, err)
	assert.Len(t, got, pagination.MaxAllRows)

	insertGroups(t, db, "over-ceiling-", 1)
	got, err = repo.ListAll(t.Context())
	require.ErrorIs(t, err, ErrTooManyRows)
	assert.Nil(t, got)
}

func TestPostgresGroupRepository_Update(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	rec := insertGroup(t, repo, "Maintainers")

	update := GroupRecord{
		ID:        rec.ID,
		Name:      "Core maintainers",
		UpdatedBy: uuid.NewV4(),
		UpdatedAt: baseTime.Add(time.Hour),
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

func TestPostgresGroupRepository_Update_NotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	_, err := repo.Update(t.Context(), newRecord("Maintainers", baseTime))

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresGroupRepository_Update_DuplicateName(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	insertGroup(t, repo, "Maintainers")
	reviewers := insertGroup(t, repo, "Reviewers")

	reviewers.Name = "Maintainers"
	_, err := repo.Update(t.Context(), reviewers)

	assert.ErrorIs(t, err, ErrDuplicateName)
}

func TestPostgresGroupRepository_Patch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		patch    *string
		wantName string
	}{
		{name: "name", patch: new("Core maintainers"), wantName: "Core maintainers"},
		{name: "none", wantName: "Maintainers"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := newTestRepo(t)
			rec := insertGroup(t, repo, "Maintainers")

			patch := GroupPatch{ID: rec.ID, Name: tt.patch, UpdatedBy: uuid.NewV4(), UpdatedAt: baseTime.Add(time.Hour)}
			want := rec
			want.Name, want.UpdatedBy, want.UpdatedAt = tt.wantName, patch.UpdatedBy, patch.UpdatedAt

			stored, err := repo.Patch(t.Context(), patch)
			require.NoError(t, err)
			assert.Equal(t, want, stored)

			got, err := repo.Get(t.Context(), rec.ID)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestPostgresGroupRepository_Patch_NotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	_, err := repo.Patch(t.Context(), GroupPatch{ID: uuid.NewV4(), Name: new("Maintainers")})

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresGroupRepository_Patch_DuplicateName(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	insertGroup(t, repo, "Maintainers")
	reviewers := insertGroup(t, repo, "Reviewers")

	_, err := repo.Patch(t.Context(), GroupPatch{ID: reviewers.ID, Name: new("Maintainers")})

	assert.ErrorIs(t, err, ErrDuplicateName)
}

func TestPostgresGroupRepository_Delete(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	rec := insertGroup(t, repo, "Maintainers")

	require.NoError(t, repo.Delete(t.Context(), rec.ID))

	_, err := repo.Get(t.Context(), rec.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresGroupRepository_Delete_RemovesMembers(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	rec := insertGroup(t, repo, "Maintainers")
	require.NoError(t, repo.InsertMember(t.Context(), newMember(rec.ID, insertIdentity(t, db), baseTime)))

	require.NoError(t, repo.Delete(t.Context(), rec.ID))

	var members int
	require.NoError(t, db.GetContext(t.Context(), &members, `SELECT count(*) FROM group_members`))
	assert.Zero(t, members, "the group's member records must be deleted with it")
}

func TestPostgresGroupRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	err := repo.Delete(t.Context(), uuid.NewV4())

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresGroupRepository_ListMembers(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	group := insertGroup(t, repo, "Maintainers")
	first := newMember(group.ID, insertIdentity(t, db), baseTime)
	second := newMember(group.ID, insertIdentity(t, db), baseTime.Add(time.Minute))
	third := newMember(group.ID, insertIdentity(t, db), baseTime.Add(2*time.Minute))
	// Inserted out of order, so the result order can only come from the creation time.
	for _, rec := range []MemberRecord{second, third, first} {
		require.NoError(t, repo.InsertMember(t.Context(), rec))
	}
	// A member of another group, which must not be listed.
	other := insertGroup(t, repo, "Reviewers")
	require.NoError(t, repo.InsertMember(t.Context(), newMember(other.ID, first.IdentityID, baseTime)))

	tests := []struct {
		name   string
		offset int
		limit  int
		want   []MemberRecord
	}{
		{name: "all", offset: 0, limit: 10, want: []MemberRecord{first, second, third}},
		{name: "limited", offset: 0, limit: 2, want: []MemberRecord{first, second}},
		{name: "offset", offset: 1, limit: 1, want: []MemberRecord{second}},
		{name: "past the end", offset: 3, limit: 10, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := repo.ListMembers(t.Context(), group.ID, tt.offset, tt.limit)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPostgresGroupRepository_ListMembers_Empty(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	group := insertGroup(t, repo, "Maintainers")

	got, err := repo.ListMembers(t.Context(), group.ID, 0, 10)
	require.NoError(t, err)

	assert.Empty(t, got)
}

func TestPostgresGroupRepository_InsertMember_KeepsExisting(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	group := insertGroup(t, repo, "Maintainers")
	rec := newMember(group.ID, insertIdentity(t, db), baseTime)
	require.NoError(t, repo.InsertMember(t.Context(), rec))

	again := newMember(group.ID, rec.IdentityID, baseTime.Add(time.Hour))
	require.NoError(t, repo.InsertMember(t.Context(), again))

	got, err := repo.ListMembers(t.Context(), group.ID, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []MemberRecord{rec}, got, "the existing member record must be kept unchanged")
}

func TestPostgresGroupRepository_InsertMember_GroupNotFound(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)

	err := repo.InsertMember(t.Context(), newMember(uuid.NewV4(), insertIdentity(t, db), baseTime))

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresGroupRepository_InsertMember_IdentityNotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	group := insertGroup(t, repo, "Maintainers")

	err := repo.InsertMember(t.Context(), newMember(group.ID, uuid.NewV4(), baseTime))

	assert.ErrorIs(t, err, ErrIdentityNotFound)
}

func TestPostgresGroupRepository_DeleteMember(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	group := insertGroup(t, repo, "Maintainers")
	kept := newMember(group.ID, insertIdentity(t, db), baseTime)
	removed := newMember(group.ID, insertIdentity(t, db), baseTime)
	for _, rec := range []MemberRecord{kept, removed} {
		require.NoError(t, repo.InsertMember(t.Context(), rec))
	}

	require.NoError(t, repo.DeleteMember(t.Context(), group.ID, removed.IdentityID))

	got, err := repo.ListMembers(t.Context(), group.ID, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []MemberRecord{kept}, got)
}

func TestPostgresGroupRepository_DeleteMember_NotFound(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	group := insertGroup(t, repo, "Maintainers")

	err := repo.DeleteMember(t.Context(), group.ID, insertIdentity(t, db))

	assert.ErrorIs(t, err, ErrMemberNotFound)
}

func TestPostgresGroupRepository_ListIdentityGroups(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	ada := insertIdentity(t, db)
	grace := insertIdentity(t, db)
	maintainers := insertGroup(t, repo, "Maintainers")
	reviewers := insertGroup(t, repo, "Reviewers")
	releasers := insertGroup(t, repo, "Releasers")
	for _, rec := range []MemberRecord{
		newMember(maintainers.ID, ada, baseTime),
		newMember(reviewers.ID, ada, baseTime),
		// Another identity's group, which must not be listed.
		newMember(releasers.ID, grace, baseTime),
	} {
		require.NoError(t, repo.InsertMember(t.Context(), rec))
	}

	got, err := repo.ListIdentityGroups(t.Context(), ada)
	require.NoError(t, err)

	assert.ElementsMatch(t, []uuid.UUID{maintainers.ID, reviewers.ID}, got)
}

func TestPostgresGroupRepository_ListIdentityGroups_None(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	insertGroup(t, repo, "Maintainers")

	for name, id := range map[string]uuid.UUID{"no groups": insertIdentity(t, db), "missing": uuid.NewV4()} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := repo.ListIdentityGroups(t.Context(), id)
			require.NoError(t, err)

			assert.Empty(t, got)
		})
	}
}
