// Package database opens connections to PostgreSQL.
package database

import (
	"context"
	"database/sql"
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

// Querier runs the queries of a repository, on the connection pool or in a transaction: *sqlx.DB and
// *sqlx.Tx implement it, so that the caller decides whether a repository takes part in a transaction.
type Querier interface {
	// GetContext scans the single row of the query into dest.
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	// SelectContext scans the rows of the query into the slice dest.
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	// ExecContext runs the query without returning rows.
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// txBeginner begins transactions, as *sqlx.DB does.
type txBeginner interface {
	BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
}

// InTx runs fn in a transaction on q. On a *sqlx.Tx, fn takes part in the transaction that q already is,
// which its owner commits or rolls back. This lets a repository that needs a transaction take part in its
// caller's. On anything that begins transactions, such as a *sqlx.DB, it begins one, which commits if fn
// returns nil and rolls back otherwise. Any other Querier is an error, without running fn, since fn would
// otherwise run without a transaction. InTx returns fn's error unchanged.
func InTx(ctx context.Context, q Querier, fn func(tx Querier) error) error {
	if tx, ok := q.(*sqlx.Tx); ok {
		return fn(tx)
	}

	db, ok := q.(txBeginner)
	if !ok {
		return fmt.Errorf("run in transaction: %T neither is a transaction nor begins one", q)
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Does nothing once the transaction is committed.
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

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
