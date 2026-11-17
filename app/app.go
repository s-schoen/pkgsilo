// Package app assembles the HTTP API of pkgsilo from its features.
package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/authn"
	"pkgsilo/core/authz"
	"pkgsilo/core/mcpserver"
	"pkgsilo/core/openapi"
	"pkgsilo/core/router"
	apikeyapi "pkgsilo/feature/apikey/api"
	apikeyrepo "pkgsilo/feature/apikey/repository"
	apikeyservice "pkgsilo/feature/apikey/service"
	apikeytools "pkgsilo/feature/apikey/tools"
	groupapi "pkgsilo/feature/group/api"
	grouprepo "pkgsilo/feature/group/repository"
	groupservice "pkgsilo/feature/group/service"
	grouptools "pkgsilo/feature/group/tools"
	healthapi "pkgsilo/feature/health/api"
	healthrepo "pkgsilo/feature/health/repository"
	healthservice "pkgsilo/feature/health/service"
	identityapi "pkgsilo/feature/identity/api"
	identityrepo "pkgsilo/feature/identity/repository"
	identityservice "pkgsilo/feature/identity/service"
	identitytools "pkgsilo/feature/identity/tools"
	policyapi "pkgsilo/feature/policy/api"
	policyrepo "pkgsilo/feature/policy/repository"
	policyservice "pkgsilo/feature/policy/service"
	policytools "pkgsilo/feature/policy/tools"
)

const (
	// apiPath is the path under which the API is mounted.
	apiPath = "/api/v1"
	// healthPath is the path of the health check, under apiPath, like the paths of the other features below.
	healthPath     = "/health"
	identitiesPath = "/identities"
	apiKeysPath    = "/identities/{identityID}/api-keys"
	groupsPath     = "/groups"
	policiesPath   = "/policies"
	// specPath is the path of the OpenAPI document, under apiPath.
	specPath = "/openapi.json"
	// mcpPath is the path of the MCP endpoint, at the root, outside apiPath.
	mcpPath = "/mcp"
)

// NewHandler returns the HTTP API: the routes of every feature, backed by db, mounted under /api/v1
// on the router that opts configures, and the MCP endpoint at /mcp, which serves the tools of the
// features to agents. The OpenAPI document of the API is at /api/v1/openapi.json. All routes but the health
// check and the document need an authenticated request.
// API keys are hashed under the secret apiKeyPepper. Requests and errors are logged to logger, and
// metrics are recorded with meterProvider: every request but a health check is measured, and so are
// authentications and authorization decisions. Every request but a health check gets a server span from
// tracerProvider.
func NewHandler(
	opts router.Options, db *sqlx.DB, apiKeyPepper []byte, logger *slog.Logger, meterProvider metric.MeterProvider,
	tracerProvider trace.TracerProvider,
) http.Handler {
	authorizer := authz.New(db, meterProvider, tracerProvider)
	health := healthservice.NewHealthService(healthrepo.NewPostgresHealthRepository(db))
	identities := identityservice.NewIdentityService(identityrepo.NewPostgresIdentityRepository(db), authorizer)
	groups := groupservice.NewGroupService(grouprepo.NewPostgresGroupRepository(db), authorizer)
	apiKeys := apikeyservice.NewAPIKeyService(apikeyrepo.NewPostgresAPIKeyRepository(db), authorizer, apiKeyPepper)
	policies := policyservice.NewPolicyService(policyrepo.NewPostgresPolicyRepository(db), authorizer, schema())

	opts.UnmeasuredPaths = slices.Concat(opts.UnmeasuredPaths, []string{apiPath + healthPath})
	authenticate := authn.Require(logger, meterProvider, tracerProvider, apikeyapi.NewAuthenticator(apiKeys))
	r := router.New(opts, logger, meterProvider, tracerProvider)
	r.With(authenticate).Handle(mcpPath, mcpserver.NewHandler(logger, meterProvider, tracerProvider,
		identitytools.New(identities),
		grouptools.New(groups),
		policytools.New(policies),
		apikeytools.New(apiKeys),
	))
	r.Route(apiPath, func(r chi.Router) {
		r.Mount(healthPath, healthapi.NewHandler(health, logger).Routes())
		r.Get(specPath, Docs().ServeHTTP)
		r.Group(func(r chi.Router) {
			r.Use(authenticate)
			r.Mount(identitiesPath, identityapi.NewHandler(identities, logger).Routes())
			r.Mount(apiKeysPath, apikeyapi.NewHandler(apiKeys, logger).Routes())
			r.Mount(groupsPath, groupapi.NewHandler(groups, logger).Routes())
			r.Mount(policiesPath, policyapi.NewHandler(policies, logger).Routes())
		})
	})

	return r
}

// Docs returns the OpenAPI document of the routes of NewHandler under /api/v1, but for the document and its
// UI. It panics if a feature describes its routes wrongly, a bug found when the API is built.
func Docs() *openapi.Document {
	spec := openapi.Spec{
		Info: openapi.Info{
			Title:   "pkgsilo API",
			Version: "v1",
			Description: "The HTTP API of pkgsilo. Responses are JSON, in a resource or collection envelope, and " +
				"problems are RFC 9457 Problem Details. Policies decide what the caller may do: an object that the " +
				"caller may not read is answered like a missing one, and fields that it may not read are absent.",
		},
		Security: apikeyapi.SecurityScheme(),
	}
	spec.MountPublic(apiPath+healthPath, healthapi.Docs())
	spec.Mount(apiPath+identitiesPath, identityapi.Docs())
	spec.Mount(apiPath+apiKeysPath, apikeyapi.Docs())
	spec.Mount(apiPath+groupsPath, groupapi.Docs())
	spec.Mount(apiPath+policiesPath, policyapi.Docs())

	doc, err := spec.Build()
	if err != nil {
		panic(fmt.Sprintf("app: %v", err))
	}

	return doc
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
