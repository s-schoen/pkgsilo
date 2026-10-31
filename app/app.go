// Package app assembles the HTTP API of pkgsilo from its features.
package app

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"

	"pkgsilo/core/router"
	healthapi "pkgsilo/health/api"
	healthrepo "pkgsilo/health/repository"
	healthservice "pkgsilo/health/service"
	identityapi "pkgsilo/identity/api"
	identityrepo "pkgsilo/identity/repository"
	identityservice "pkgsilo/identity/service"
)

// NewHandler returns the HTTP API: the routes of every feature, backed by db, mounted under /api/v1
// on the router that opts configures. Requests and errors are logged to logger.
func NewHandler(opts router.Options, db *sqlx.DB, logger *slog.Logger) http.Handler {
	health := healthservice.NewHealthService(healthrepo.NewPostgresHealthRepository(db))
	identities := identityservice.NewIdentityService(identityrepo.NewPostgresIdentityRepository(db))

	r := router.New(opts, logger)
	r.Route("/api/v1", func(r chi.Router) {
		r.Mount("/health", healthapi.NewHandler(health, logger).Routes())
		r.Mount("/identities", identityapi.NewHandler(identities, logger).Routes())
	})

	return r
}
