// Package repository reaches the stores that pkgsilo depends on, to check their health.
package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// HealthRepository reaches the stores that pkgsilo depends on.
type HealthRepository interface {
	// Ping verifies that the database accepts queries.
	Ping(ctx context.Context) error
}

// PostgresHealthRepository reaches the PostgreSQL database.
type PostgresHealthRepository struct {
	db *sqlx.DB
}

var _ HealthRepository = (*PostgresHealthRepository)(nil)

// NewPostgresHealthRepository returns a PostgresHealthRepository that reaches db.
func NewPostgresHealthRepository(db *sqlx.DB) *PostgresHealthRepository {
	return &PostgresHealthRepository{db: db}
}

// Ping verifies that the database accepts queries.
func (r *PostgresHealthRepository) Ping(ctx context.Context) error {
	if err := r.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return nil
}
