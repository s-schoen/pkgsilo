package repository

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/pagination"
	"pkgsilo/test/databasetest"
)

// seededPolicies is the number of policies that the migrations create.
const seededPolicies = 2

// administratorsGroup is the ID of the group that the migrations create.
var administratorsGroup = uuid.MustParse("7c1d3e52-8a4f-4b6e-9f21-5d0c8e7a3b14")

// newTestRepo returns a repository on a new database, which holds the seeded policies alone.
func newTestRepo(t *testing.T) (*PostgresPolicyRepository, *sqlx.DB) {
	t.Helper()

	db := databasetest.New(t)

	return NewPostgresPolicyRepository(db), db
}

// newRecord returns a record created at the given time. Times have microsecond precision, like PostgreSQL
// timestamps, so records survive a round trip unchanged.
func newRecord(name string, createdAt time.Time) PolicyRecord {
	actor := uuid.NewV4()

	return PolicyRecord{
		ID:        uuid.NewV4(),
		Name:      name,
		Source:    `permit (principal, action, resource);`,
		CreatedBy: actor,
		UpdatedBy: actor,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

// The seeded policies were created at the time of the migration, before every record of these tests.
var baseTime = time.Now().Add(time.Hour).Truncate(time.Microsecond).UTC()

// revision returns the current policy revision.
func revision(t *testing.T, db *sqlx.DB) int64 {
	t.Helper()

	var rev int64
	require.NoError(t, db.GetContext(t.Context(), &rev, `SELECT revision FROM policy_revision`))

	return rev
}

// insert stores the records, each in a write of its own.
func insert(t *testing.T, repo *PostgresPolicyRepository, recs ...PolicyRecord) {
	t.Helper()

	for _, rec := range recs {
		err := repo.Write(t.Context(), func(w PolicyWriter) error {
			_, err := w.Insert(t.Context(), rec)
			return err
		})
		require.NoError(t, err)
	}
}

// userPolicies returns the records without the seeded ones.
func userPolicies(t *testing.T, recs []PolicyRecord) []PolicyRecord {
	t.Helper()
	require.GreaterOrEqual(t, len(recs), seededPolicies)

	return recs[seededPolicies:]
}

func TestPostgresPolicyRepository_InsertAndGet(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	rec := newRecord("everything", baseTime)

	var stored PolicyRecord
	err := repo.Write(t.Context(), func(w PolicyWriter) error {
		var err error
		stored, err = w.Insert(t.Context(), rec)
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, rec, stored)

	got, err := repo.Get(t.Context(), rec.ID)
	require.NoError(t, err)
	assert.Equal(t, rec, got)
}

func TestPostgresPolicyRepository_Insert_DuplicateName(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	insert(t, repo, newRecord("everything", baseTime))

	err := repo.Write(t.Context(), func(w PolicyWriter) error {
		_, err := w.Insert(t.Context(), newRecord("everything", baseTime))
		return err
	})

	assert.ErrorIs(t, err, ErrDuplicateName)
}

func TestPostgresPolicyRepository_Get_NotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	_, err := repo.Get(t.Context(), uuid.NewV4())

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresPolicyRepository_ListAll(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	first := newRecord("first", baseTime)
	second := newRecord("second", baseTime.Add(time.Minute))
	third := newRecord("third", baseTime.Add(2*time.Minute))
	// Inserted out of order, so the result order can only come from the creation time.
	insert(t, repo, second, third, first)

	got, err := repo.ListAll(t.Context())
	require.NoError(t, err)

	assert.Equal(t, []string{"administrators", "read-own-identity"},
		[]string{got[0].Name, got[1].Name}, "the seeded policies come first")
	assert.Equal(t, []PolicyRecord{first, second, third}, userPolicies(t, got))
}

func TestPostgresPolicyRepository_ListAll_Ceiling(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)

	insertPolicies := func(prefix string, count int) {
		_, err := db.ExecContext(t.Context(), `
			INSERT INTO policies (id, name, source, created_by, updated_by, created_at, updated_at)
			SELECT gen_random_uuid(), $1 || n, '', $2::uuid, $2::uuid, $3::timestamptz, $3::timestamptz
			FROM generate_series(1, $4::int) AS n`,
			prefix, uuid.NewV4(), baseTime, count,
		)
		require.NoError(t, err)
	}

	insertPolicies("at-ceiling-", pagination.MaxAllRows-seededPolicies)
	got, err := repo.ListAll(t.Context())
	require.NoError(t, err)
	assert.Len(t, got, pagination.MaxAllRows)

	insertPolicies("over-ceiling-", 1)
	got, err = repo.ListAll(t.Context())
	require.ErrorIs(t, err, ErrTooManyRows)
	assert.Nil(t, got)

	// The writer lists the same way.
	err = repo.Write(t.Context(), func(w PolicyWriter) error {
		_, err := w.ListAll(t.Context())
		return err
	})
	assert.ErrorIs(t, err, ErrTooManyRows)
}

func TestPostgresPolicyRepository_ListExistingGroups(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	group := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO groups (id, name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, 'maintainers', $1, $1, $2, $2)`,
		group, baseTime,
	)
	require.NoError(t, err)

	tests := []struct {
		name string
		ids  []uuid.UUID
		want []uuid.UUID
	}{
		{
			name: "some exist",
			ids:  []uuid.UUID{group, uuid.NewV4(), administratorsGroup},
			want: []uuid.UUID{group, administratorsGroup},
		},
		{name: "none exists", ids: []uuid.UUID{uuid.NewV4()}},
		{name: "no IDs", ids: []uuid.UUID{}},
		{name: "nil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := repo.ListExistingGroups(t.Context(), tt.ids)
			require.NoError(t, err)

			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestPostgresPolicyRepository_Update(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	rec := newRecord("everything", baseTime)
	insert(t, repo, rec)

	update := PolicyRecord{
		ID:        rec.ID,
		Name:      "nothing",
		Source:    `forbid (principal, action, resource);`,
		UpdatedBy: uuid.NewV4(),
		UpdatedAt: baseTime.Add(time.Hour),
		// Immutable: must not overwrite the stored values.
		CreatedBy: uuid.NewV4(),
		CreatedAt: baseTime.Add(time.Hour),
	}
	want := update
	want.CreatedBy, want.CreatedAt = rec.CreatedBy, rec.CreatedAt

	var stored PolicyRecord
	err := repo.Write(t.Context(), func(w PolicyWriter) error {
		var err error
		stored, err = w.Update(t.Context(), update)
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, want, stored)

	got, err := repo.Get(t.Context(), rec.ID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestPostgresPolicyRepository_Update_Errors(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	insert(t, repo, newRecord("everything", baseTime))
	nothing := newRecord("nothing", baseTime)
	insert(t, repo, nothing)

	tests := []struct {
		name    string
		rec     PolicyRecord
		wantErr error
	}{
		{name: "not found", rec: newRecord("missing", baseTime), wantErr: ErrNotFound},
		{name: "duplicate name", rec: PolicyRecord{ID: nothing.ID, Name: "everything"}, wantErr: ErrDuplicateName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := repo.Write(t.Context(), func(w PolicyWriter) error {
				_, err := w.Update(t.Context(), tt.rec)
				return err
			})

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestPostgresPolicyRepository_Delete(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	rec := newRecord("everything", baseTime)
	insert(t, repo, rec)

	err := repo.Write(t.Context(), func(w PolicyWriter) error {
		return w.Delete(t.Context(), rec.ID)
	})
	require.NoError(t, err)

	_, err = repo.Get(t.Context(), rec.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresPolicyRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	err := repo.Write(t.Context(), func(w PolicyWriter) error {
		return w.Delete(t.Context(), uuid.NewV4())
	})

	assert.ErrorIs(t, err, ErrNotFound)
}

// TestPostgresPolicyRepository_Write_AdvancesRevision checks that a write that commits advances the
// revision once, and that one that fails leaves the revision and the records as they were.
func TestPostgresPolicyRepository_Write_AdvancesRevision(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	before := revision(t, db)

	insert(t, repo, newRecord("everything", baseTime))
	assert.Equal(t, before+1, revision(t, db))

	errFailed := errors.New("check failed")
	rec := newRecord("nothing", baseTime)
	err := repo.Write(t.Context(), func(w PolicyWriter) error {
		if _, err := w.Insert(t.Context(), rec); err != nil {
			return err
		}
		return errFailed
	})
	require.ErrorIs(t, err, errFailed, "fn's error must be returned")

	assert.Equal(t, before+1, revision(t, db), "a failed write must not advance the revision")
	_, err = repo.Get(t.Context(), rec.ID)
	assert.ErrorIs(t, err, ErrNotFound, "a failed write must roll back")
}

// TestPostgresPolicyRepository_Write_Serialized checks that a write waits for another one to commit, so that
// it sees what the other one wrote.
func TestPostgresPolicyRepository_Write_Serialized(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	first := newRecord("first", baseTime)

	inFirst := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- repo.Write(context.WithoutCancel(t.Context()), func(w PolicyWriter) error {
			if _, err := w.Insert(t.Context(), first); err != nil {
				return err
			}
			close(inFirst)
			<-release
			return nil
		})
	}()
	<-inFirst

	secondDone := make(chan []PolicyRecord, 1)
	go func() {
		_ = repo.Write(t.Context(), func(w PolicyWriter) error {
			recs, err := w.ListAll(t.Context())
			secondDone <- recs
			return err
		})
	}()

	select {
	case <-secondDone:
		t.Fatal("the second write must wait for the first one to commit")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-firstDone)

	assert.Equal(t, []PolicyRecord{first}, userPolicies(t, <-secondDone), "the second write must see the first one")
}
