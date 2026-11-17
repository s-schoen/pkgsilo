package repository

import (
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/test/databasetest"
)

var baseTime = time.Date(2026, time.November, 17, 12, 0, 0, 123456000, time.UTC)

const (
	// passwordHash is the hash of the password of every identity that insertIdentity stores.
	passwordHash = "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$a2V5"
	// lockNotAvailable is the PostgreSQL error code of a lock timeout.
	lockNotAvailable = "55P03"
)

// newTestRepo returns a repository on a new, empty database, and the database for setting up the
// identities that records refer to.
func newTestRepo(t *testing.T) (*PostgresSessionRepository, *sqlx.DB) {
	t.Helper()

	db := databasetest.New(t)

	return NewPostgresSessionRepository(db), db
}

// insertIdentity stores an identity row with a password of passwordHash, for records to refer to, and returns
// its ID. The identities and passwords tables belong to the identity and password features, so the rows are
// written directly.
func insertIdentity(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()

	id := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO identities (id, username, display_name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, 'Owner', $1, $1, $3, $3)`,
		id, "owner-"+id.String(), baseTime,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(),
		`INSERT INTO passwords (identity_id, hash, set_at) VALUES ($1, $2, $3)`, id, passwordHash, baseTime)
	require.NoError(t, err)

	return id
}

// newRecord returns a record of the identity created and last seen at baseTime, which expires a day later,
// with a unique hash. Times have microsecond precision, like PostgreSQL timestamps, so records survive a
// round trip unchanged.
func newRecord(identityID uuid.UUID) SessionRecord {
	id := uuid.NewV4()

	return SessionRecord{
		ID:         id,
		IdentityID: identityID,
		TokenHash:  id[:],
		CreatedAt:  baseTime,
		LastSeenAt: baseTime,
		ExpiresAt:  baseTime.Add(24 * time.Hour),
		ClientIP:   new("192.0.2.1"),
		UserAgent:  "Mozilla/5.0",
	}
}

// insertRecord stores rec and returns it.
func insertRecord(t *testing.T, repo *PostgresSessionRepository, rec SessionRecord) SessionRecord {
	t.Helper()

	stored, err := repo.Insert(t.Context(), rec, passwordHash)
	require.NoError(t, err)

	return stored
}

// assertGone asserts that the record is no longer stored.
func assertGone(t *testing.T, repo *PostgresSessionRepository, rec SessionRecord) {
	t.Helper()

	_, err := repo.GetByHash(t.Context(), rec.TokenHash)
	assert.ErrorIs(t, err, ErrNotFound)
}

// assertStored asserts that the record is still stored.
func assertStored(t *testing.T, repo *PostgresSessionRepository, rec SessionRecord) {
	t.Helper()

	_, err := repo.GetByHash(t.Context(), rec.TokenHash)
	assert.NoError(t, err)
}

func TestPostgresSessionRepository_InsertAndGet(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	withIPv6 := newRecord(insertIdentity(t, db))
	withIPv6.ClientIP = new("2001:db8::1")
	withoutIP := newRecord(insertIdentity(t, db))
	withoutIP.ClientIP = nil

	tests := []struct {
		name string
		rec  SessionRecord
	}{
		{name: "IPv4", rec: newRecord(insertIdentity(t, db))},
		{name: "IPv6", rec: withIPv6},
		{name: "unknown IP", rec: withoutIP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stored, err := repo.Insert(t.Context(), tt.rec, passwordHash)
			require.NoError(t, err)
			assert.Equal(t, tt.rec, stored)

			got, err := repo.Get(t.Context(), tt.rec.IdentityID, tt.rec.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.rec, got)
		})
	}
}

// TestPostgresSessionRepository_Insert_PasswordChanged checks that no session is stored once the password that
// its login verified is gone, so that a login can't outlast a change of the password.
func TestPostgresSessionRepository_Insert_PasswordChanged(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	changed, deleted := insertIdentity(t, db), insertIdentity(t, db)
	_, err := db.ExecContext(t.Context(), `UPDATE passwords SET hash = 'changed' WHERE identity_id = $1`, changed)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `DELETE FROM passwords WHERE identity_id = $1`, deleted)
	require.NoError(t, err)

	tests := []struct {
		name       string
		identityID uuid.UUID
	}{
		{name: "changed since", identityID: changed},
		{name: "deleted since", identityID: deleted},
		{name: "unknown identity", identityID: uuid.NewV4()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := newRecord(tt.identityID)

			_, err := repo.Insert(t.Context(), rec, passwordHash)

			require.ErrorIs(t, err, ErrPasswordChanged)
			assertGone(t, repo, rec)
		})
	}
}

// TestPostgresSessionRepository_Insert_LocksPassword checks that a change of the password waits for an insert
// that holds it, so that the change comes after the session and ends it with the others.
func TestPostgresSessionRepository_Insert_LocksPassword(t *testing.T) {
	t.Parallel()
	_, db := newTestRepo(t)
	identityID := insertIdentity(t, db)
	login, err := db.BeginTxx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = login.Rollback() })
	_, err = NewPostgresSessionRepository(login).Insert(t.Context(), newRecord(identityID), passwordHash)
	require.NoError(t, err)

	change, err := db.BeginTxx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = change.Rollback() })
	_, err = change.ExecContext(t.Context(), `SET LOCAL lock_timeout = '100ms'`)
	require.NoError(t, err)
	_, err = change.ExecContext(t.Context(), `UPDATE passwords SET hash = 'changed' WHERE identity_id = $1`, identityID)

	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	require.True(t, ok, "want a lock timeout, got %v", err)
	assert.Equal(t, lockNotAvailable, pgErr.Code)
}

func TestPostgresSessionRepository_Insert_DuplicateHashFails(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db)
	rec := insertRecord(t, repo, newRecord(identityID))
	duplicate := newRecord(identityID)
	duplicate.TokenHash = rec.TokenHash

	_, err := repo.Insert(t.Context(), duplicate, passwordHash)

	assert.Error(t, err)
}

func TestPostgresSessionRepository_Get_NotFound(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID, otherID := insertIdentity(t, db), insertIdentity(t, db)
	rec := insertRecord(t, repo, newRecord(identityID))

	tests := []struct {
		name       string
		identityID uuid.UUID
		id         uuid.UUID
	}{
		{name: "unknown id", identityID: identityID, id: uuid.NewV4()},
		{name: "session of another identity", identityID: otherID, id: rec.ID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := repo.Get(t.Context(), tt.identityID, tt.id)

			assert.ErrorIs(t, err, ErrNotFound)
		})
	}
}

func TestPostgresSessionRepository_GetByHash(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	rec := insertRecord(t, repo, newRecord(insertIdentity(t, db)))

	got, err := repo.GetByHash(t.Context(), rec.TokenHash)
	require.NoError(t, err)

	assert.Equal(t, rec, got)
}

func TestPostgresSessionRepository_GetByHash_NotFound(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	_, err := repo.GetByHash(t.Context(), []byte("unknown"))

	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostgresSessionRepository_UpdateLastSeen(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	rec := insertRecord(t, repo, newRecord(insertIdentity(t, db)))
	other := insertRecord(t, repo, newRecord(insertIdentity(t, db)))
	lastSeenAt := baseTime.Add(time.Hour)

	require.NoError(t, repo.UpdateLastSeen(t.Context(), rec.ID, lastSeenAt))

	got, err := repo.GetByHash(t.Context(), rec.TokenHash)
	require.NoError(t, err)
	rec.LastSeenAt = lastSeenAt
	assert.Equal(t, rec, got)
	unchanged, err := repo.GetByHash(t.Context(), other.TokenHash)
	require.NoError(t, err)
	assert.Equal(t, other, unchanged)
}

func TestPostgresSessionRepository_UpdateLastSeen_Missing(t *testing.T) {
	t.Parallel()
	repo, _ := newTestRepo(t)

	assert.NoError(t, repo.UpdateLastSeen(t.Context(), uuid.NewV4(), baseTime))
}

func TestPostgresSessionRepository_Delete(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db)
	rec := insertRecord(t, repo, newRecord(identityID))
	other := insertRecord(t, repo, newRecord(identityID))

	require.NoError(t, repo.Delete(t.Context(), identityID, rec.ID))

	assertGone(t, repo, rec)
	assertStored(t, repo, other)
}

func TestPostgresSessionRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	rec := insertRecord(t, repo, newRecord(insertIdentity(t, db)))

	tests := []struct {
		name       string
		identityID uuid.UUID
		id         uuid.UUID
	}{
		{name: "unknown id", identityID: rec.IdentityID, id: uuid.NewV4()},
		{name: "session of another identity", identityID: insertIdentity(t, db), id: rec.ID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := repo.Delete(t.Context(), tt.identityID, tt.id)

			assert.ErrorIs(t, err, ErrNotFound)
		})
	}
	assertStored(t, repo, rec)
}

func TestPostgresSessionRepository_DeleteByHash(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db)
	rec := insertRecord(t, repo, newRecord(identityID))
	other := insertRecord(t, repo, newRecord(identityID))

	require.NoError(t, repo.DeleteByHash(t.Context(), rec.TokenHash))
	require.NoError(t, repo.DeleteByHash(t.Context(), rec.TokenHash), "a missing session is no error")

	assertGone(t, repo, rec)
	assertStored(t, repo, other)
}

func TestPostgresSessionRepository_DeleteByIdentity(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db)
	first := insertRecord(t, repo, newRecord(identityID))
	second := insertRecord(t, repo, newRecord(identityID))
	other := insertRecord(t, repo, newRecord(insertIdentity(t, db)))

	require.NoError(t, repo.DeleteByIdentity(t.Context(), identityID))
	require.NoError(t, repo.DeleteByIdentity(t.Context(), identityID), "an identity without sessions is no error")

	assertGone(t, repo, first)
	assertGone(t, repo, second)
	assertStored(t, repo, other)
}

func TestPostgresSessionRepository_DeleteExpired(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	identityID := insertIdentity(t, db)
	now := baseTime.Add(2 * time.Hour)
	idleSince := now.Add(-time.Hour)
	// with returns a record with the given last use and expiry.
	with := func(lastSeenAt, expiresAt time.Time) SessionRecord {
		rec := newRecord(identityID)
		rec.LastSeenAt, rec.ExpiresAt = lastSeenAt, expiresAt

		return insertRecord(t, repo, rec)
	}
	active := with(idleSince.Add(time.Microsecond), now.Add(time.Microsecond))
	idle := with(idleSince, now.Add(time.Hour))
	expired := with(now, now)

	require.NoError(t, repo.DeleteExpired(t.Context(), now, idleSince))

	assertStored(t, repo, active)
	assertGone(t, repo, idle)
	assertGone(t, repo, expired)
}

func TestPostgresSessionRepository_IdentityDeleteCascades(t *testing.T) {
	t.Parallel()
	repo, db := newTestRepo(t)
	rec := insertRecord(t, repo, newRecord(insertIdentity(t, db)))

	_, err := db.ExecContext(t.Context(), `DELETE FROM identities WHERE id = $1`, rec.IdentityID)
	require.NoError(t, err)

	assertGone(t, repo, rec)
}
