// Package app assembles the HTTP API of pkgsilo from its features.
package app

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel/metric"

	"pkgsilo/core/authn"
	"pkgsilo/core/authz"
	"pkgsilo/core/router"
	apikeyapi "pkgsilo/feature/apikey/api"
	apikeyrepo "pkgsilo/feature/apikey/repository"
	apikeyservice "pkgsilo/feature/apikey/service"
	groupapi "pkgsilo/feature/group/api"
	grouprepo "pkgsilo/feature/group/repository"
	groupservice "pkgsilo/feature/group/service"
	healthapi "pkgsilo/feature/health/api"
	healthrepo "pkgsilo/feature/health/repository"
	healthservice "pkgsilo/feature/health/service"
	identityapi "pkgsilo/feature/identity/api"
	identityrepo "pkgsilo/feature/identity/repository"
	identityservice "pkgsilo/feature/identity/service"
	policyapi "pkgsilo/feature/policy/api"
	policyrepo "pkgsilo/feature/policy/repository"
	policyservice "pkgsilo/feature/policy/service"
)

const (
	// apiPath is the path under which the API is mounted.
	apiPath = "/api/v1"
	// healthPath is the path of the health check, under apiPath.
	healthPath = "/health"
)

// NewHandler returns the HTTP API: the routes of every feature, backed by db, mounted under /api/v1
// on the router that opts configures. All routes but the health check need an authenticated request.
// API keys are hashed under the secret apiKeyPepper. Requests and errors are logged to logger, and
// metrics are recorded with meterProvider: every request but a health check is measured.
func NewHandler(
	opts router.Options, db *sqlx.DB, apiKeyPepper []byte, logger *slog.Logger, meterProvider metric.MeterProvider,
) http.Handler {
	authorizer := authz.New(db)
	health := healthservice.NewHealthService(healthrepo.NewPostgresHealthRepository(db))
	identities := identityservice.NewIdentityService(identityrepo.NewPostgresIdentityRepository(db), authorizer)
	groups := groupservice.NewGroupService(grouprepo.NewPostgresGroupRepository(db), authorizer)
	apiKeys := apikeyservice.NewAPIKeyService(apikeyrepo.NewPostgresAPIKeyRepository(db), authorizer, apiKeyPepper)
	policies := policyservice.NewPolicyService(policyrepo.NewPostgresPolicyRepository(db), authorizer, schema())

	opts.MeterProvider = meterProvider
	opts.UnmeasuredPaths = slices.Concat(opts.UnmeasuredPaths, []string{apiPath + healthPath})
	r := router.New(opts, logger)
	r.Route(apiPath, func(r chi.Router) {
		r.Mount(healthPath, healthapi.NewHandler(health, logger).Routes())
		r.Group(func(r chi.Router) {
			r.Use(authn.Require(logger, apikeyapi.NewAuthenticator(apiKeys)))
			r.Mount("/identities", identityapi.NewHandler(identities, logger).Routes())
			r.Mount("/identities/{identityID}/api-keys", apikeyapi.NewHandler(apiKeys, logger).Routes())
			r.Mount("/groups", groupapi.NewHandler(groups, logger).Routes())
			r.Mount("/policies", policyapi.NewHandler(policies, logger).Routes())
		})
	})

	return r
}

// schema returns the Cedar schema that saved policies are validated against: the resource types and
// actions that the features declare.
func schema() string {
	return authz.Schema(
		identityservice.Actions().Declaration(),
		groupservice.Actions().Declaration(),
		apikeyservice.Actions().Declaration(),
		policyservice.Actions().Declaration(),
	)
}
