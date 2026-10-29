package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"pkgsilo/core/config"
)

func TestOpen(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a PostgreSQL container")
	}

	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:18-alpine", postgres.BasicWaitStrategies())
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err)

	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := Open(ctx, config.Config{DatabaseURL: url, DatabaseMaxConns: 3})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var n int
	require.NoError(t, db.GetContext(ctx, &n, "SELECT 1"))
	assert.Equal(t, 1, n)
	assert.Equal(t, 3, db.Stats().MaxOpenConnections)

	var ts time.Time
	require.NoError(t, db.GetContext(ctx, &ts, "SELECT '2026-10-24 12:00:00+02'::timestamptz"))
	assert.Equal(t, time.UTC, ts.Location())
	assert.Equal(t, time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC), ts)
}

func TestOpen_InvalidURL(t *testing.T) {
	_, err := Open(context.Background(), config.Config{DatabaseURL: "postgres://:invalid", DatabaseMaxConns: 1})
	assert.ErrorContains(t, err, "parse database url")
}

func TestOpen_Unreachable(t *testing.T) {
	cfg := config.Config{DatabaseURL: "postgres://u:secret@127.0.0.1:1/db?connect_timeout=1", DatabaseMaxConns: 1}

	_, err := Open(context.Background(), cfg)
	require.ErrorContains(t, err, "ping database")
	assert.NotContains(t, err.Error(), "secret")
}
