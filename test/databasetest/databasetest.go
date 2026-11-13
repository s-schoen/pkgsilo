// Package databasetest provides migrated PostgreSQL databases for repository and end-to-end tests.
// It must import nothing from pkgsilo but core/database, so that internal tests of the features'
// packages can use it without an import cycle.
package databasetest

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/database"
)

const (
	// templateDatabase holds the migrated schema. Nothing may connect to it, since
	// PostgreSQL refuses to clone a template that has open sessions.
	templateDatabase = "pkgsilo_template"
	buildDatabase    = "pkgsilo_template_build"
	adminDatabase    = "postgres"
	// templateLockKey is the advisory lock that guards building the template.
	templateLockKey = 7_421_001
	readyTimeout    = 30 * time.Second
)

var (
	// setupServer runs setup once per test process, the first time a test needs a database.
	setupServer = sync.OnceValue(setup)
	adminDB     *sqlx.DB
	serverURL   *url.URL
)

// New returns a connection to a new database with all migrations applied, so that it holds nothing but
// the migrations' seeded data. The database is dropped when the test ends. It records no metrics and
// creates no spans. New skips the test in short mode.
func New(t *testing.T) *sqlx.DB {
	t.Helper()

	return NewWithProviders(t, noop.NewMeterProvider(), tracenoop.NewTracerProvider())
}

// NewWithProviders is like New, but the connection records its pool and query metrics with meterProvider
// and creates the spans of its queries with tracerProvider, as database.Open does.
func NewWithProviders(
	t *testing.T, meterProvider metric.MeterProvider, tracerProvider trace.TracerProvider,
) *sqlx.DB {
	t.Helper()

	if testing.Short() {
		t.Skip("needs a PostgreSQL container")
	}

	if err := setupServer(); err != nil {
		t.Fatalf("set up test database server: %v", err)
	}

	name := fmt.Sprintf("test_%x", randomBytes())
	// The name is generated, so it's safe to format into the statement.
	_, err := adminDB.ExecContext(t.Context(), fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, templateDatabase))
	if err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		// t.Context() is already canceled when cleanups run, so keep only its values.
		ctx := context.WithoutCancel(t.Context())
		_, err := adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", name))
		if err != nil {
			t.Errorf("drop test database: %v", err)
		}
	})

	db, err := database.Open(t.Context(), database.Options{URL: databaseURL(name), MaxConns: 2},
		meterProvider, tracerProvider)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	// Registered after the drop, so it runs before it.
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// setup starts the PostgreSQL container, connects to its admin database and makes sure the
// template database exists.
//
// All test packages of a `go test` run share one container: every package runs in its own
// process, and the session ID is derived from the parent `go test` process. The container
// is never terminated here, since other packages may still use it; Ryuk removes it when
// the session ends.
func setup() error {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.BasicWaitStrategies(),
		testcontainers.WithReuseByName("pkgsilo-test-"+testcontainers.SessionID()),
	)
	if err != nil {
		return fmt.Errorf("start postgres: %w", err)
	}

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("connection string: %w", err)
	}
	serverURL, err = url.Parse(connStr)
	if err != nil {
		return fmt.Errorf("parse connection string: %w", err)
	}

	adminDB, err = openWhenReady(ctx, databaseURL(adminDatabase))
	if err != nil {
		return fmt.Errorf("open admin database: %w", err)
	}

	return createTemplate(ctx)
}

// createTemplate creates the template database with all up migrations applied, unless
// another test process already did. An advisory lock lets only one process build it.
// The migrations are applied over SQL rather than as init scripts, since copying the
// scripts into the container crashes the Podman API service when several packages start
// at once.
func createTemplate(ctx context.Context) error {
	// The advisory lock belongs to the session, so it needs a dedicated connection.
	conn, err := adminDB.Connx(ctx)
	if err != nil {
		return fmt.Errorf("get admin connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", templateLockKey); err != nil {
		return fmt.Errorf("lock template: %w", err)
	}
	defer func() { _, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", templateLockKey) }()

	var exists bool
	err = conn.GetContext(ctx, &exists, "SELECT EXISTS (SELECT FROM pg_database WHERE datname = $1)", templateDatabase)
	if err != nil {
		return fmt.Errorf("check template: %w", err)
	}
	if exists {
		return nil
	}

	// The template is built under another name and renamed when complete, so a process
	// that dies halfway never leaves a partial template behind.
	for _, stmt := range []string{
		fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", buildDatabase),
		"CREATE DATABASE " + buildDatabase,
	} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("create template: %w", err)
		}
	}
	if err := migrate(ctx, databaseURL(buildDatabase)); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, fmt.Sprintf("ALTER DATABASE %s RENAME TO %s", buildDatabase, templateDatabase))
	if err != nil {
		return fmt.Errorf("rename template: %w", err)
	}

	return nil
}

// migrate applies all up migrations, in order, to the database at dbURL.
func migrate(ctx context.Context, dbURL string) error {
	migrations, err := filepath.Glob(filepath.Join(migrationsDir(), "*.up.sql"))
	if err != nil {
		return fmt.Errorf("find migrations: %w", err)
	}
	if len(migrations) == 0 {
		return errors.New("find migrations: no up migrations")
	}

	db, err := database.Open(ctx, database.Options{URL: dbURL, MaxConns: 1},
		noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	if err != nil {
		return fmt.Errorf("open template: %w", err)
	}
	defer func() { _ = db.Close() }()

	for _, path := range migrations {
		stmts, err := os.ReadFile(path) //nolint:gosec // path comes from globbing the migrations directory.
		if err != nil {
			return fmt.Errorf("read migration: %w", err)
		}
		// Without arguments, pgx sends the file as one simple query, so it may hold several statements.
		if _, err := db.ExecContext(ctx, string(stmts)); err != nil {
			return fmt.Errorf("apply migration %s: %w", filepath.Base(path), err)
		}
	}

	return nil
}

// openWhenReady opens the database at dbURL, retrying until the server accepts connections.
// A reused container skips the wait strategies, so it may still be starting.
func openWhenReady(ctx context.Context, dbURL string) (*sqlx.DB, error) {
	deadline := time.Now().Add(readyTimeout)
	for {
		db, err := database.Open(ctx, database.Options{URL: dbURL, MaxConns: 4},
			noop.NewMeterProvider(), tracenoop.NewTracerProvider())
		if err == nil {
			return db, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("not ready after %s: %w", readyTimeout, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// databaseURL returns the URL of the named database on the test server.
func databaseURL(name string) string {
	u := serverURL.Clone()
	u.Path = "/" + name

	return u.String()
}

// RunMigration applies the migration file of the given name to db, for tests of what a migration does to
// a database that already holds data: such a test migrates down, sets up the data and migrates up again.
func RunMigration(t *testing.T, db *sqlx.DB, name string) {
	t.Helper()

	stmts, err := os.ReadFile(filepath.Join(migrationsDir(), name)) //nolint:gosec // name comes from the test.
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.ExecContext(t.Context(), string(stmts)); err != nil {
		t.Fatalf("apply migration %s: %v", name, err)
	}
}

// migrationsDir returns the repository's migrations directory. It's resolved from this
// file, since tests run in the directory of the package under test.
func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func randomBytes() []byte {
	b := make([]byte, 8)
	_, _ = rand.Read(b)

	return b
}
