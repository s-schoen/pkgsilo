package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"pkgsilo/core/config"
)

const (
	databasePingTimeout     = 5 * time.Second
	databaseConnMaxIdleTime = 5 * time.Minute
)

// Open connects to PostgreSQL at cfg.DatabaseURL and verifies the connection.
// The caller must close the returned database.
func Open(ctx context.Context, cfg config.Config) (*sqlx.DB, error) {
	// pgx redacts the password in parse errors, so the error is safe to log.
	connCfg, err := pgx.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	db := sqlx.NewDb(stdlib.OpenDB(*connCfg), "pgx")
	db.SetMaxOpenConns(cfg.DatabaseMaxConns)
	db.SetMaxIdleConns(cfg.DatabaseMaxConns)
	db.SetConnMaxIdleTime(databaseConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, databasePingTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}
