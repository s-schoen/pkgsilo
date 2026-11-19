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
	"pkgsilo/core/middleware"
	"pkgsilo/core/openapi"
	"pkgsilo/core/router"
	"pkgsilo/core/spa"
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
	passwordapi "pkgsilo/feature/password/api"
	passwordrepo "pkgsilo/feature/password/repository"
	passwordservice "pkgsilo/feature/password/service"
	passwordtools "pkgsilo/feature/password/tools"
	policyapi "pkgsilo/feature/policy/api"
	policyrepo "pkgsilo/feature/policy/repository"
	policyservice "pkgsilo/feature/policy/service"
	policytools "pkgsilo/feature/policy/tools"
	sessionapi "pkgsilo/feature/session/api"
	sessionrepo "pkgsilo/feature/session/repository"
	sessionservice "pkgsilo/feature/session/service"
	"pkgsilo/ui"
)

const (
	// apiPath is the path under which the API is mounted.
	apiPath = "/api/v1"
	// healthPath is the path of the health check, under apiPath, like the paths of the other features below.
	healthPath     = "/health"
	identitiesPath = "/identities"
	apiKeysPath    = "/identities/{identityID}/api-keys"
	passwordPath   = "/identities/{identityID}/password"
	sessionsPath   = "/sessions"
	groupsPath     = "/groups"
	policiesPath   = "/policies"
	// specPath is the path of the OpenAPI document, under apiPath.
	specPath = "/openapi.json"
	// mcpPath is the path of the MCP endpoint, at the root, outside apiPath.
	mcpPath = "/mcp"
	// uiPath is the pattern of the web UI, which gets every path that no other route takes.
	uiPath = "/*"
)

// NewHandler returns the HTTP API: the routes of every feature, backed by db, mounted under /api/v1
// on the router that opts configures, and the MCP endpoint at /mcp, which serves the tools of the
// features to agents. Every other path serves the web UI of package ui, which needs no authentication. The
// OpenAPI document of the API is at /api/v1/openapi.json. All routes but the health check, the document and
// the UI need an authenticated request. The routes under /api/v1 reject state-changing
// requests that a browser sends from another origin, and no response is stored unless its handler says so.
// API keys are hashed under the secret apiKeyPepper. A login with a password creates a session, whose cookie
// authenticates the routes under /api/v1, but not /mcp, until it exceeds sessionLifetimes. Requests and errors
// are logged to logger, and metrics are recorded with meterProvider: every request but a health check is
// measured, and so are authentications and authorization decisions. Every request but a health check gets a
// server span from tracerProvider. It panics if the web UI can't be read, a bug of the build.
func NewHandler(
	opts router.Options, db *sqlx.DB, apiKeyPepper []byte, sessionLifetimes sessionservice.Lifetimes,
	logger *slog.Logger, meterProvider metric.MeterProvider, tracerProvider trace.TracerProvider,
) http.Handler {
	authorizer := authz.New(db, meterProvider, tracerProvider)
	owners := authz.NewOwners(db, identityservice.Actions().Read())
	health := healthservice.NewHealthService(healthrepo.NewPostgresHealthRepository(db))
	identities := identityservice.NewIdentityService(identityrepo.NewPostgresIdentityRepository(db), authorizer)
	groups := groupservice.NewGroupService(grouprepo.NewPostgresGroupRepository(db), authorizer)
	apiKeys := apikeyservice.NewAPIKeyService(apikeyrepo.NewPostgresAPIKeyRepository(db), authorizer, owners,
		apiKeyPepper)
	passwordRepo := passwordrepo.NewPostgresPasswordRepository(db)
	sessions := sessionservice.NewSessionService(sessionrepo.NewPostgresSessionRepository(db),
		passwordservice.NewVerifier(passwordRepo), sessionLifetimes)
	// The verifier, then the sessions, then the passwords, which end the sessions of an identity whose password
	// changes.
	passwords := passwordservice.NewPasswordService(passwordRepo, authorizer, owners, sessions)
	policies := policyservice.NewPolicyService(policyrepo.NewPostgresPolicyRepository(db), authorizer, schema())

	opts.UnmeasuredPaths = slices.Concat(opts.UnmeasuredPaths, []string{apiPath + healthPath})
	// API keys first, so that an API key wins when a request carries a session cookie too. Only the API takes
	// the cookie: MCP clients aren't browsers.
	authenticateMCP := authn.Require(logger, meterProvider, tracerProvider, apikeyapi.NewAuthenticator(apiKeys))
	authenticate := authn.Require(logger, meterProvider, tracerProvider,
		apikeyapi.NewAuthenticator(apiKeys), sessionapi.NewAuthenticator(sessions))
	logins := authn.NewLoginRecorder(meterProvider, tracerProvider, "password")
	r := router.New(opts, logger, meterProvider, tracerProvider)
	r.With(authenticateMCP).Handle(mcpPath, mcpserver.NewHandler(logger, meterProvider, tracerProvider,
		identitytools.New(identities),
		grouptools.New(groups),
		policytools.New(policies),
		apikeytools.New(apiKeys),
		passwordtools.New(passwords),
	))
	r.Route(apiPath, func(r chi.Router) {
		// Only the API: MCP clients aren't browsers, and the session cookie authenticates only the API.
		r.Use(middleware.CrossOriginProtection)
		r.Mount(healthPath, healthapi.NewHandler(health, logger).Routes())
		r.Get(specPath, Docs().ServeHTTP)
		// The login is public, and the routes of the current session are behind authenticate.
		r.Mount(sessionsPath, sessionapi.NewHandler(sessions, logins, logger).Routes(authenticate))
		r.Group(func(r chi.Router) {
			r.Use(authenticate)
			r.Mount(identitiesPath, identityapi.NewHandler(identities, logger).Routes())
			r.Mount(apiKeysPath, apikeyapi.NewHandler(apiKeys, logger).Routes())
			r.Mount(passwordPath, passwordapi.NewHandler(passwords, logger).Routes())
			r.Mount(groupsPath, groupapi.NewHandler(groups, logger).Routes())
			r.Mount(policiesPath, policyapi.NewHandler(policies, logger).Routes())
		})
	})
	// After the API, which keeps the 404 problem of router.New for a path under /api/v1 that it doesn't route.
	// The UI's own 404 is the same problem, and the router answers the other methods with its 405.
	webUI, err := spa.Handler(ui.FS(), r.NotFoundHandler())
	if err != nil {
		// The UI is embedded in the binary, so it can't be read only if the build is broken.
		panic(fmt.Sprintf("app: %v", err))
	}
	r.Method(http.MethodGet, uiPath, webUI)
	r.Method(http.MethodHead, uiPath, webUI)

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
		Security:             []openapi.SecurityScheme{apikeyapi.SecurityScheme(), sessionapi.SecurityScheme()},
		CrossOriginProtected: true,
	}
	spec.MountPublic(apiPath+healthPath, healthapi.Docs())
	spec.Mount(apiPath+identitiesPath, identityapi.Docs())
	spec.Mount(apiPath+apiKeysPath, apikeyapi.Docs())
	spec.Mount(apiPath+passwordPath, passwordapi.Docs())
	spec.Mount(apiPath+sessionsPath, sessionapi.Docs())
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
		passwordservice.Actions().Declaration(),
		policyservice.Actions().Declaration(),
	)
}
