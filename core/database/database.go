// Package database opens connections to PostgreSQL.
package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
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
//
// fn runs its queries with the context it's passed. When InTx begins a transaction inside a span, that
// context holds the span transactionSpanName, under which BEGIN, fn's queries and COMMIT nest. The span
// comes from the tracer provider of the span in ctx, since InTx takes none, so that outside a traced
// request, such as in app.Bootstrap, there's no span. On a *sqlx.Tx, fn gets ctx as it is.
func InTx(ctx context.Context, q Querier, fn func(ctx context.Context, tx Querier) error) error {
	if tx, ok := q.(*sqlx.Tx); ok {
		return fn(ctx, tx)
	}

	db, ok := q.(txBeginner)
	if !ok {
		return fmt.Errorf("run in transaction: %T neither is a transaction nor begins one", q)
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer(tracerName).Start(ctx, transactionSpanName)
	defer span.End()

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		// No transaction began, so the span has no outcome.
		span.SetStatus(codes.Error, "begin transaction")

		return fmt.Errorf("begin transaction: %w", err)
	}
	// Does nothing once the transaction is committed.
	defer func() { _ = tx.Rollback() }()
	// The outcome until COMMIT succeeds. A rollback because fn failed isn't an error of the transaction, so
	// only BEGIN and COMMIT set the status.
	span.SetAttributes(outcomeKey.String(outcomeRollback))

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		span.SetStatus(codes.Error, "commit transaction")

		return fmt.Errorf("commit transaction: %w", err)
	}
	span.SetAttributes(outcomeKey.String(outcomeCommit))

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
//
// The pool and every operation on its connections are measured with meterProvider: see
// registerPoolMetrics for the pool, and otelpgx for db.client.operation.duration and
// db.client.operation.errors, with its default attributes only, so no query text reaches a metric.
// Closing the database stops the pool metrics.
//
// otelpgx creates the spans of the connections' operations with tracerProvider, with its defaults: each is
// named after its operation, such as SELECT, and holds the query text and the connection details, but never
// the query's parameters, which would carry secrets such as API key hashes. otelpgx starts a span only
// under a recording one, so that queries outside a traced request, such as those of app.Bootstrap or of
// the connections that untraced health checks open, start no trace of their own.
func Open(
	ctx context.Context, opts Options, meterProvider metric.MeterProvider, tracerProvider trace.TracerProvider,
) (*sqlx.DB, error) {
	// pgx redacts the password in parse errors, so the error is safe to log.
	connCfg, err := pgx.ParseConfig(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	connCfg.Tracer = otelpgx.NewTracer(
		otelpgx.WithMeterProvider(meterProvider),
		otelpgx.WithTracerProvider(tracerProvider),
	)

	connector := &closingConnector{
		Connector: stdlib.GetConnector(*connCfg, stdlib.OptionAfterConnect(scanTimestamptzInUTC)),
	}
	db := sqlx.NewDb(sql.OpenDB(connector), "pgx")
	db.SetMaxOpenConns(opts.MaxConns)
	db.SetMaxIdleConns(opts.MaxConns)
	db.SetConnMaxIdleTime(databaseConnMaxIdleTime)

	connector.onClose, err = registerPoolMetrics(meterProvider.Meter(meterName), db.DB, poolName(connCfg))
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, databasePingTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

// closingConnector is a driver.Connector that calls onClose, if set, when the *sql.DB that it connects
// closes: sql.DB.Close closes a connector that implements io.Closer.
type closingConnector struct {
	driver.Connector

	onClose func() error
}

// Close calls onClose, if set.
func (c *closingConnector) Close() error {
	if c.onClose == nil {
		return nil
	}

	return c.onClose()
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
