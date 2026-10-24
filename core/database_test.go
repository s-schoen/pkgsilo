package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestOpenDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a PostgreSQL container")
	}

	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:18-alpine", postgres.BasicWaitStrategies())
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err)

	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := OpenDatabase(ctx, Config{DatabaseURL: url, DatabaseMaxConns: 3})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var n int
	require.NoError(t, db.GetContext(ctx, &n, "SELECT 1"))
	assert.Equal(t, 1, n)
	assert.Equal(t, 3, db.Stats().MaxOpenConnections)
}

func TestOpenDatabase_InvalidURL(t *testing.T) {
	_, err := OpenDatabase(context.Background(), Config{DatabaseURL: "postgres://:invalid", DatabaseMaxConns: 1})
	assert.ErrorContains(t, err, "parse database url")
}

func TestOpenDatabase_Unreachable(t *testing.T) {
	cfg := Config{DatabaseURL: "postgres://u:secret@127.0.0.1:1/db?connect_timeout=1", DatabaseMaxConns: 1}

	_, err := OpenDatabase(context.Background(), cfg)
	require.ErrorContains(t, err, "ping database")
	assert.NotContains(t, err.Error(), "secret")
}
