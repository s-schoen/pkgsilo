package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/database"
	"pkgsilo/test/databasetest"
	"pkgsilo/test/telemetrytest"
)

// newTracedScratchTable is like newScratchTable, but the database creates its spans with rec's provider.
// The table is created outside a span, so that it adds no span of its own.
func newTracedScratchTable(t *testing.T, rec *telemetrytest.Recorder) *sqlx.DB {
	t.Helper()

	db := databasetest.NewWithProviders(t, rec.MeterProvider(), rec.TracerProvider())
	_, err := db.ExecContext(t.Context(), `CREATE TABLE scratch (n int NOT NULL UNIQUE DEFERRABLE INITIALLY DEFERRED)`)
	require.NoError(t, err)

	return db
}

// startParent starts a span on rec's provider, as the server span of a request would be, and returns the
// context that holds it.
func startParent(t *testing.T, rec *telemetrytest.Recorder) (context.Context, trace.Span) {
	t.Helper()

	return rec.TracerProvider().Tracer("pkgsilo/test").Start(t.Context(), "parent")
}

// transactionSpan returns the only ended transaction span, failing the test if there's not exactly one.
func transactionSpan(t *testing.T, spans []sdktrace.ReadOnlySpan) sdktrace.ReadOnlySpan {
	t.Helper()

	var found []sdktrace.ReadOnlySpan
	for _, span := range spans {
		if span.Name() == "pkgsilo.database.transaction" {
			found = append(found, span)
		}
	}
	require.Len(t, found, 1, "transaction spans")

	return found[0]
}

// children returns the spans whose parent is parent, in the order in which they ended.
func children(spans []sdktrace.ReadOnlySpan, parent trace.SpanContext) []sdktrace.ReadOnlySpan {
	var children []sdktrace.ReadOnlySpan
	for _, span := range spans {
		if span.Parent().SpanID() == parent.SpanID() {
			children = append(children, span)
		}
	}

	return children
}

// childNames returns the names of the spans whose parent is parent, in the order in which they ended.
func childNames(spans []sdktrace.ReadOnlySpan, parent trace.SpanContext) []string {
	var names []string
	for _, span := range children(spans, parent) {
		names = append(names, span.Name())
	}

	return names
}

// TestOpen_QuerySpans pins the defaults of otelpgx: a query in a span gets a child span named after its
// operation, with its text and the connection details, but without its parameters. The first run of a
// query also prepares it, in a child span of the query's, named the same.
func TestOpen_QuerySpans(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := databasetest.NewWithProviders(t, rec.MeterProvider(), rec.TracerProvider())
	var namespace string
	require.NoError(t, db.GetContext(t.Context(), &namespace, `SELECT current_database()`))
	const query = `SELECT $1::text`

	ctx, parent := startParent(t, rec)
	var got string
	require.NoError(t, db.GetContext(ctx, &got, query, "secret"))
	parent.End()

	spans := rec.Spans()
	queries := children(spans, parent.SpanContext())
	require.Len(t, queries, 1)
	span := queries[0]
	assert.Equal(t, "SELECT", span.Name())
	assert.Equal(t, trace.SpanKindClient, span.SpanKind())
	attrs := attribute.NewSet(span.Attributes()...)
	for key, want := range map[attribute.Key]attribute.Value{
		"db.system.name": attribute.StringValue("postgresql"),
		"db.query.text":  attribute.StringValue(query),
		"db.namespace":   attribute.StringValue(namespace),
	} {
		value, _ := attrs.Value(key)
		assert.Equal(t, want, value, key)
	}
	for _, key := range []attribute.Key{"server.address", "server.port", "user.name"} {
		assert.True(t, attrs.HasValue(key), key)
	}
	for _, span := range spans {
		for _, kv := range span.Attributes() {
			assert.NotEqual(t, attribute.Key("pgx.query.parameters"), kv.Key, span.Name())
			assert.NotContains(t, kv.Value.String(), "secret", "%s holds the parameter in %s", span.Name(), kv.Key)
		}
	}
}

// TestOpen_NoSpansWithoutParent checks that neither a query nor the connect of a new connection starts a
// trace of its own when its context holds no span.
func TestOpen_NoSpansWithoutParent(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := databasetest.NewWithProviders(t, rec.MeterProvider(), rec.TracerProvider())
	// Holds the connection that Open pinged with, so that the query needs a new one.
	held, err := db.Conn(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() { _ = held.Close() })

	var n int
	require.NoError(t, db.GetContext(t.Context(), &n, `SELECT 1`))

	assert.Equal(t, 2, db.Stats().OpenConnections, "the query must have connected")
	assert.Empty(t, rec.Spans())
}

func TestInTx_SpanCommit(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := newTracedScratchTable(t, rec)

	ctx, parent := startParent(t, rec)
	require.NoError(t, database.InTx(ctx, db, insertRow(t, nil)))
	parent.End()

	spans := rec.Spans()
	tx := transactionSpan(t, spans)
	assert.Equal(t, parent.SpanContext().SpanID(), tx.Parent().SpanID())
	assert.Equal(t, []attribute.KeyValue{attribute.String("pkgsilo.database.transaction.outcome", "commit")},
		tx.Attributes())
	assert.Equal(t, codes.Unset, tx.Status().Code)
	assert.Equal(t, []string{"BEGIN", "INSERT", "COMMIT"}, childNames(spans, tx.SpanContext()))
}

// TestInTx_SpanRollback checks that a transaction that rolls back because fn failed is no error of the
// transaction.
func TestInTx_SpanRollback(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := newTracedScratchTable(t, rec)

	ctx, parent := startParent(t, rec)
	require.Error(t, database.InTx(ctx, db, insertRow(t, errors.New("failed"))))
	parent.End()

	spans := rec.Spans()
	tx := transactionSpan(t, spans)
	assert.Equal(t, []attribute.KeyValue{attribute.String("pkgsilo.database.transaction.outcome", "rollback")},
		tx.Attributes())
	assert.Equal(t, codes.Unset, tx.Status().Code)
	assert.Equal(t, []string{"BEGIN", "INSERT", "ROLLBACK"}, childNames(spans, tx.SpanContext()))
}

// TestInTx_SpanCommitFails checks that a COMMIT that fails, here on a deferred constraint, is an error of
// the transaction, which rolled back.
func TestInTx_SpanCommitFails(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := newTracedScratchTable(t, rec)

	ctx, parent := startParent(t, rec)
	err := database.InTx(ctx, db, func(ctx context.Context, tx database.Querier) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO scratch (n) VALUES (1), (1)`)
		return err
	})
	parent.End()

	require.ErrorContains(t, err, "commit transaction")
	tx := transactionSpan(t, rec.Spans())
	assert.Equal(t, []attribute.KeyValue{attribute.String("pkgsilo.database.transaction.outcome", "rollback")},
		tx.Attributes())
	assert.Equal(t, sdktrace.Status{Code: codes.Error, Description: "commit transaction"}, tx.Status())
}

// TestInTx_SpanBeginFails checks that a BEGIN that fails, here on a canceled context, is an error of the
// transaction.
func TestInTx_SpanBeginFails(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := newTracedScratchTable(t, rec)

	ctx, parent := startParent(t, rec)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	err := database.InTx(canceled, db, insertRow(t, nil))
	parent.End()

	require.ErrorIs(t, err, context.Canceled)
	tx := transactionSpan(t, rec.Spans())
	assert.Equal(t, []attribute.KeyValue{attribute.String("pkgsilo.database.transaction.outcome", "rollback")},
		tx.Attributes())
	assert.Equal(t, sdktrace.Status{Code: codes.Error, Description: "begin transaction"}, tx.Status())
}

// TestInTx_SpanTxTakesPart checks that fn taking part in the caller's transaction starts no transaction
// span: the caller's transaction is no new one.
func TestInTx_SpanTxTakesPart(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := newTracedScratchTable(t, rec)

	ctx, parent := startParent(t, rec)
	tx, err := db.BeginTxx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, database.InTx(ctx, tx, insertRow(t, nil)))
	require.NoError(t, tx.Commit())
	parent.End()

	spans := rec.Spans()
	for _, span := range spans {
		assert.NotEqual(t, "pkgsilo.database.transaction", span.Name())
	}
	assert.Equal(t, []string{"BEGIN", "INSERT", "COMMIT"}, childNames(spans, parent.SpanContext()))
}

// TestInTx_SpanWithoutParent checks that a transaction outside a span, as in app.Bootstrap, starts no trace.
func TestInTx_SpanWithoutParent(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := newTracedScratchTable(t, rec)

	require.NoError(t, database.InTx(t.Context(), db, insertRow(t, nil)))

	assert.Empty(t, rec.Spans())
}
