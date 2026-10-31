package repository

import (
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/test/databasetest"
)

func TestPostgresHealthRepository_Ping(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)

	assert.NoError(t, NewPostgresHealthRepository(db).Ping(t.Context()))
}

func TestPostgresHealthRepository_PingUnreachable(t *testing.T) {
	t.Parallel()
	// sqlx.Open doesn't connect, so the unreachable server is only noticed by Ping.
	db, err := sqlx.Open("pgx", "postgres://u@127.0.0.1:1/db?connect_timeout=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	assert.ErrorContains(t, NewPostgresHealthRepository(db).Ping(t.Context()), "ping database")
}
