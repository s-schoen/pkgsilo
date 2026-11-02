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

// newTestRepo returns a repository on a new, empty database, and the database for setting up the
// identities that member records refer to.
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

func TestPostgresGroupRepository_List(t *testing.T) {
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

	tests := []struct {
		name   string
		offset int
		limit  int
		want   []GroupRecord
	}{
		{name: "all", offset: 0, limit: 10, want: []GroupRecord{first, second, third}},
		{name: "limited", offset: 0, limit: 2, want: []GroupRecord{first, second}},
		{name: "offset", offset: 1, limit: 1, want: []GroupRecord{second}},
		{name: "past the end", offset: 3, limit: 10, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := repo.List(t.Context(), tt.offset, tt.limit)
			require.NoError(t, err)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPostgresGroupRepository_List_Empty(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	got, err := repo.List(t.Context(), 0, 10)
	require.NoError(t, err)

	assert.Empty(t, got)
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
