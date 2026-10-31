// Package database opens connections to PostgreSQL.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	databasePingTimeout     = 5 * time.Second
	databaseConnMaxIdleTime = 5 * time.Minute
)

// Options configures the database connection.
type Options struct {
	// URL is the PostgreSQL connection string, e.g. postgres://user:pass@host:5432/db.
	URL string
	// MaxConns caps the open connections in the pool.
	MaxConns int
}

// Open connects to PostgreSQL at opts.URL and verifies the connection. Scanned
// timestamptz values are in UTC. The caller must close the returned database.
func Open(ctx context.Context, opts Options) (*sqlx.DB, error) {
	// pgx redacts the password in parse errors, so the error is safe to log.
	connCfg, err := pgx.ParseConfig(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	db := sqlx.NewDb(stdlib.OpenDB(*connCfg, stdlib.OptionAfterConnect(scanTimestamptzInUTC)), "pgx")
	db.SetMaxOpenConns(opts.MaxConns)
	db.SetMaxIdleConns(opts.MaxConns)
	db.SetConnMaxIdleTime(databaseConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, databasePingTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

// scanTimestamptzInUTC makes conn return timestamptz values in UTC instead of the local
// time zone.
func scanTimestamptzInUTC(_ context.Context, conn *pgx.Conn) error {
	conn.TypeMap().RegisterType(&pgtype.Type{
		Name:  "timestamptz",
		OID:   pgtype.TimestamptzOID,
		Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC},
	})

	return nil
}
