// Package repository persists sessions.
package repository

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"pkgsilo/core/database"
)

// SessionRepository stores session records.
type SessionRepository interface {
	// Insert stores a new record and returns it as stored.
	Insert(ctx context.Context, rec SessionRecord) (SessionRecord, error)
	// Get returns the record with the given ID of the given identity.
	Get(ctx context.Context, identityID, id uuid.UUID) (SessionRecord, error)
	// GetByHash returns the record with the given token hash, whatever its identity.
	GetByHash(ctx context.Context, tokenHash []byte) (SessionRecord, error)
	// UpdateLastSeen sets the last use of the record with the given ID. A record that no longer exists is
	// not an error: its session ended while it was in use.
	UpdateLastSeen(ctx context.Context, id uuid.UUID, lastSeenAt time.Time) error
	// Delete removes the record with the given ID of the given identity.
	Delete(ctx context.Context, identityID, id uuid.UUID) error
	// DeleteByHash removes the record with the given token hash, if there is one.
	DeleteByHash(ctx context.Context, tokenHash []byte) error
	// DeleteExpired removes every record that expires at or before now, or was last seen at or before
	// idleSince.
	DeleteExpired(ctx context.Context, now, idleSince time.Time) error
}

// sessionColumns are the columns of a record. The client IP is read in its text form without a prefix
// length, so that it parses as an address.
const (
	sessionColumns = "id, identity_id, token_hash, created_at, last_seen_at, expires_at, client_ip, user_agent"
	selectColumns  = "id, identity_id, token_hash, created_at, last_seen_at, expires_at, host(client_ip) AS client_ip, " +
		"user_agent"
)

// PostgresSessionRepository stores session records in the sessions table of PostgreSQL.
type PostgresSessionRepository struct {
	db database.Querier
}

var _ SessionRepository = (*PostgresSessionRepository)(nil)

// NewPostgresSessionRepository returns a PostgresSessionRepository that stores records in db.
func NewPostgresSessionRepository(db database.Querier) *PostgresSessionRepository {
	return &PostgresSessionRepository{db: db}
}

// Insert stores a new record and returns it as stored.
func (r *PostgresSessionRepository) Insert(ctx context.Context, rec SessionRecord) (SessionRecord, error) {
	var stored SessionRecord
	err := r.db.GetContext(ctx, &stored, `
		INSERT INTO sessions (`+sessionColumns+`)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+selectColumns,
		rec.ID, rec.IdentityID, rec.TokenHash, rec.CreatedAt, rec.LastSeenAt, rec.ExpiresAt, rec.ClientIP,
		rec.UserAgent,
	)
	if err != nil {
		return SessionRecord{}, fmt.Errorf("insert session: %w", translateError(err))
	}

	return stored, nil
}

// Get returns the record with the given ID of the given identity.
func (r *PostgresSessionRepository) Get(ctx context.Context, identityID, id uuid.UUID) (SessionRecord, error) {
	var rec SessionRecord
	err := r.db.GetContext(ctx, &rec,
		`SELECT `+selectColumns+` FROM sessions WHERE identity_id = $1 AND id = $2`, identityID, id)
	if err != nil {
		return SessionRecord{}, fmt.Errorf("get session: %w", translateError(err))
	}

	return rec, nil
}

// GetByHash returns the record with the given token hash, whatever its identity.
func (r *PostgresSessionRepository) GetByHash(ctx context.Context, tokenHash []byte) (SessionRecord, error) {
	var rec SessionRecord
	err := r.db.GetContext(ctx, &rec, `SELECT `+selectColumns+` FROM sessions WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return SessionRecord{}, fmt.Errorf("get session by hash: %w", translateError(err))
	}

	return rec, nil
}

// UpdateLastSeen sets the last use of the record with the given ID. A record that no longer exists is not an
// error: its session ended while it was in use.
func (r *PostgresSessionRepository) UpdateLastSeen(ctx context.Context, id uuid.UUID, lastSeenAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = $2 WHERE id = $1`, id, lastSeenAt)
	if err != nil {
		return fmt.Errorf("update session last seen: %w", err)
	}

	return nil
}

// Delete removes the record with the given ID of the given identity.
func (r *PostgresSessionRepository) Delete(ctx context.Context, identityID, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE identity_id = $1 AND id = $2`, identityID, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("delete session: %w", ErrNotFound)
	}

	return nil
}

// DeleteByHash removes the record with the given token hash, if there is one.
func (r *PostgresSessionRepository) DeleteByHash(ctx context.Context, tokenHash []byte) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash); err != nil {
		return fmt.Errorf("delete session by hash: %w", err)
	}

	return nil
}

// DeleteExpired removes every record that expires at or before now, or was last seen at or before idleSince,
// in one statement.
func (r *PostgresSessionRepository) DeleteExpired(ctx context.Context, now, idleSince time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at <= $1 OR last_seen_at <= $2`, now, idleSince)
	if err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}

	return nil
}
