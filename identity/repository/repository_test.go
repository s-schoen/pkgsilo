package repository

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"pkgsilo/core/config"
	"pkgsilo/core/database"
)

// testDB is shared by all tests of the package. It is nil in short mode.
var testDB *sqlx.DB

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	code, err := runWithDatabase(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}

// runWithDatabase runs the tests against a PostgreSQL container with all up migrations applied.
func runWithDatabase(m *testing.M) (int, error) {
	ctx := context.Background()

	migrations, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil {
		return 0, fmt.Errorf("find migrations: %w", err)
	}

	ctr, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithOrderedInitScripts(migrations...),
		postgres.BasicWaitStrategies(),
	)
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()
	if err != nil {
		return 0, fmt.Errorf("start postgres: %w", err)
	}

	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return 0, fmt.Errorf("connection string: %w", err)
	}

	testDB, err = database.Open(ctx, config.Config{DatabaseURL: url, DatabaseMaxConns: 2})
	if err != nil {
		return 0, fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = testDB.Close() }()

	return m.Run(), nil
}

// newTestRepo returns a repository on an empty identities table.
func newTestRepo(t *testing.T) *PostgresIdentityRepository {
	t.Helper()

	if testing.Short() {
		t.Skip("needs a PostgreSQL container")
	}

	_, err := testDB.ExecContext(t.Context(), "TRUNCATE identities")
	require.NoError(t, err)

	return NewPostgresIdentityRepository(testDB)
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

var baseTime = time.Date(2026, 10, 24, 12, 0, 0, 123456000, time.UTC)

// inUTC returns rec with its timestamps in UTC, since PostgreSQL returns them in local time.
func inUTC(rec IdentityRecord) IdentityRecord {
	rec.CreatedAt = rec.CreatedAt.UTC()
	rec.UpdatedAt = rec.UpdatedAt.UTC()

	return rec
}

func inUTCAll(recs []IdentityRecord) []IdentityRecord {
	for i := range recs {
		recs[i] = inUTC(recs[i])
	}

	return recs
}

func TestPostgresIdentityRepository_InsertAndGet(t *testing.T) {
	repo := newTestRepo(t)
	rec := newRecord("ada", baseTime)

	stored, err := repo.Insert(t.Context(), rec)
	require.NoError(t, err)
	assert.Equal(t, rec, inUTC(stored))

	got, err := repo.Get(t.Context(), rec.ID)
	require.NoError(t, err)
	assert.Equal(t, rec, inUTC(got))
}

func TestPostgresIdentityRepository_Insert_DuplicateUsername(t *testing.T) {
	repo := newTestRepo(t)
	_, err := repo.Insert(t.Context(), newRecord("ada", baseTime))
	require.NoError(t, err)

	_, err = repo.Insert(t.Context(), newRecord("ada", baseTime))

	assert.ErrorIs(t, err, ErrDuplicateUsername)
}

func TestPostgresIdentityRepository_Get_NotFound(t *testing.T) {
	repo := newTestRepo(t)

	_, err := repo.Get(t.Context(), uuid.NewV4())

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresIdentityRepository_List(t *testing.T) {
	repo := newTestRepo(t)
	first := newRecord("first", baseTime)
	second := newRecord("second", baseTime.Add(time.Minute))
	third := newRecord("third", baseTime.Add(2*time.Minute))
	// Inserted out of order, so the result order can only come from the creation time.
	for _, rec := range []IdentityRecord{second, third, first} {
		_, err := repo.Insert(t.Context(), rec)
		require.NoError(t, err)
	}

	tests := []struct {
		name   string
		offset int
		limit  int
		want   []IdentityRecord
	}{
		{name: "all", offset: 0, limit: 10, want: []IdentityRecord{first, second, third}},
		{name: "limited", offset: 0, limit: 2, want: []IdentityRecord{first, second}},
		{name: "offset", offset: 1, limit: 1, want: []IdentityRecord{second}},
		{name: "past the end", offset: 3, limit: 10, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.List(t.Context(), tt.offset, tt.limit)
			require.NoError(t, err)

			assert.Equal(t, tt.want, inUTCAll(got))
		})
	}
}

func TestPostgresIdentityRepository_List_Empty(t *testing.T) {
	repo := newTestRepo(t)

	got, err := repo.List(t.Context(), 0, 10)
	require.NoError(t, err)

	assert.Empty(t, got)
}

func TestPostgresIdentityRepository_Update(t *testing.T) {
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
	assert.Equal(t, want, inUTC(stored))

	got, err := repo.Get(t.Context(), rec.ID)
	require.NoError(t, err)
	assert.Equal(t, want, inUTC(got))
}

func TestPostgresIdentityRepository_Update_NotFound(t *testing.T) {
	repo := newTestRepo(t)

	_, err := repo.Update(t.Context(), newRecord("ada", baseTime))

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresIdentityRepository_Update_DuplicateUsername(t *testing.T) {
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
	repo := newTestRepo(t)
	rec := newRecord("ada", baseTime)
	_, err := repo.Insert(t.Context(), rec)
	require.NoError(t, err)

	require.NoError(t, repo.Delete(t.Context(), rec.ID))

	_, err = repo.Get(t.Context(), rec.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresIdentityRepository_Delete_NotFound(t *testing.T) {
	repo := newTestRepo(t)

	err := repo.Delete(t.Context(), uuid.NewV4())

	assert.ErrorIs(t, err, ErrNotFound)
}
