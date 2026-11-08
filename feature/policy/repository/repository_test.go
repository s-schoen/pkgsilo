package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/test/databasetest"
)

// seededPolicies is the number of policies that the migrations create.
const seededPolicies = 2

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

	assert.Equal(t, []string{"read-own-identity", "manage-own-api-keys"},
		[]string{got[0].Name, got[1].Name}, "the seeded policies come first")
	assert.Equal(t, []PolicyRecord{first, second, third}, userPolicies(t, got))
}

func TestPostgresPolicyRepository_ListExistingGroups(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	group, other := uuid.NewV4(), uuid.NewV4()
	for i, id := range []uuid.UUID{group, other} {
		_, err := db.ExecContext(t.Context(), `
			INSERT INTO groups (id, name, created_by, updated_by, created_at, updated_at)
			VALUES ($1, $2, $1, $1, $3, $3)`,
			id, []string{"maintainers", "auditors"}[i], baseTime,
		)
		require.NoError(t, err)
	}

	tests := []struct {
		name string
		ids  []uuid.UUID
		want []uuid.UUID
	}{
		{
			name: "some exist",
			ids:  []uuid.UUID{group, uuid.NewV4(), other},
			want: []uuid.UUID{group, other},
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

// patch patches a record in a write of its own and returns it as stored.
func patch(t *testing.T, repo *PostgresPolicyRepository, p PolicyPatch) (PolicyRecord, error) {
	t.Helper()

	var stored PolicyRecord
	err := repo.Write(t.Context(), func(w PolicyWriter) error {
		var err error
		stored, err = w.Patch(t.Context(), p)
		return err
	})

	return stored, err
}

func TestPostgresPolicyRepository_Writer_Get(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	rec := newRecord("everything", baseTime)
	insert(t, repo, rec)

	var got PolicyRecord
	var missingErr error
	err := repo.Write(t.Context(), func(w PolicyWriter) error {
		var err error
		got, err = w.Get(t.Context(), rec.ID)
		_, missingErr = w.Get(t.Context(), uuid.NewV4())
		return err
	})

	require.NoError(t, err)
	assert.Equal(t, rec, got)
	assert.ErrorIs(t, missingErr, ErrNotFound)
}

func TestPostgresPolicyRepository_Patch(t *testing.T) {
	t.Parallel()
	const nothing = `forbid (principal, action, resource);`
	tests := []struct {
		name  string
		patch func(p *PolicyPatch)
		want  func(rec *PolicyRecord)
	}{
		{
			name:  "name",
			patch: func(p *PolicyPatch) { p.Name = new("nothing") },
			want:  func(rec *PolicyRecord) { rec.Name = "nothing" },
		},
		{
			name:  "source",
			patch: func(p *PolicyPatch) { p.Source = new(nothing) },
			want:  func(rec *PolicyRecord) { rec.Source = nothing },
		},
		{
			name:  "both",
			patch: func(p *PolicyPatch) { p.Name, p.Source = new("nothing"), new(nothing) },
			want:  func(rec *PolicyRecord) { rec.Name, rec.Source = "nothing", nothing },
		},
		{name: "none", patch: func(*PolicyPatch) {}, want: func(*PolicyRecord) {}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := newTestRepo(t)
			rec := newRecord("everything", baseTime)
			insert(t, repo, rec)

			p := PolicyPatch{ID: rec.ID, UpdatedBy: uuid.NewV4(), UpdatedAt: baseTime.Add(time.Hour)}
			tt.patch(&p)
			want := rec
			want.UpdatedBy, want.UpdatedAt = p.UpdatedBy, p.UpdatedAt
			tt.want(&want)

			stored, err := patch(t, repo, p)
			require.NoError(t, err)
			assert.Equal(t, want, stored)

			got, err := repo.Get(t.Context(), rec.ID)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

// TestPostgresPolicyRepository_Patch_Concurrent checks that concurrent patches of different fields both
// survive.
func TestPostgresPolicyRepository_Patch_Concurrent(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	rec := newRecord("everything", baseTime)
	insert(t, repo, rec)
	const nothing = `forbid (principal, action, resource);`

	patches := []PolicyPatch{
		{ID: rec.ID, Name: new("nothing"), UpdatedBy: uuid.NewV4(), UpdatedAt: baseTime},
		{ID: rec.ID, Source: new(nothing), UpdatedBy: uuid.NewV4(), UpdatedAt: baseTime},
	}
	var wg sync.WaitGroup
	errs := make([]error, len(patches))
	for i, p := range patches {
		wg.Go(func() { _, errs[i] = patch(t, repo, p) })
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}

	got, err := repo.Get(t.Context(), rec.ID)
	require.NoError(t, err)
	assert.Equal(t, "nothing", got.Name)
	assert.Equal(t, nothing, got.Source)
}

func TestPostgresPolicyRepository_Patch_Errors(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)
	insert(t, repo, newRecord("everything", baseTime))
	nothing := newRecord("nothing", baseTime)
	insert(t, repo, nothing)

	tests := []struct {
		name    string
		patch   PolicyPatch
		wantErr error
	}{
		{name: "not found", patch: PolicyPatch{ID: uuid.NewV4(), Name: new("missing")}, wantErr: ErrNotFound},
		{name: "duplicate name", patch: PolicyPatch{ID: nothing.ID, Name: new("everything")}, wantErr: ErrDuplicateName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := patch(t, repo, tt.patch)

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

// TestPostgresPolicyRepository_Write_InTransaction checks that a repository of a transaction writes in it:
// its writes, and the revision it advances, are visible in the transaction alone, and go with it when the
// transaction rolls back, even though the write succeeded.
func TestPostgresPolicyRepository_Write_InTransaction(t *testing.T) {
	t.Parallel()
	_, db := newTestRepo(t)
	before := revision(t, db)
	tx, err := db.BeginTxx(t.Context(), nil)
	require.NoError(t, err)
	repo := NewPostgresPolicyRepository(tx)
	rec := newRecord("everything", baseTime)

	insert(t, repo, rec)

	got, err := repo.Get(t.Context(), rec.ID)
	require.NoError(t, err, "the transaction must see the write")
	assert.Equal(t, rec, got)
	_, err = NewPostgresPolicyRepository(db).Get(t.Context(), rec.ID)
	require.ErrorIs(t, err, ErrNotFound, "nothing may commit before the transaction does")

	require.NoError(t, tx.Rollback())
	_, err = NewPostgresPolicyRepository(db).Get(t.Context(), rec.ID)
	require.ErrorIs(t, err, ErrNotFound)
	assert.Equal(t, before, revision(t, db), "the revision must roll back with the transaction")
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
