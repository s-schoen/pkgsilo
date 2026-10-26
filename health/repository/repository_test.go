package repository

import (
	"context"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"pkgsilo/core/config"
	"pkgsilo/core/database"
)

func TestPostgresHealthRepository_Ping(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a PostgreSQL container")
	}

	ctx := context.Background()

	ctr, err := postgres.Run(ctx, "postgres:18-alpine", postgres.BasicWaitStrategies())
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err)

	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := database.Open(ctx, config.Config{DatabaseURL: url, DatabaseMaxConns: 1})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	assert.NoError(t, NewPostgresHealthRepository(db).Ping(ctx))
}

func TestPostgresHealthRepository_PingUnreachable(t *testing.T) {
	// sqlx.Open doesn't connect, so the unreachable server is only noticed by Ping.
	db, err := sqlx.Open("pgx", "postgres://u@127.0.0.1:1/db?connect_timeout=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	assert.ErrorContains(t, NewPostgresHealthRepository(db).Ping(t.Context()), "ping database")
}
