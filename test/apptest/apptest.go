// Package apptest serves the whole application to end-to-end tests.
//
// It depends on every feature through app, so only the end-to-end tests at the root of a feature
// can import it, not the tests of the feature's layer packages.
package apptest

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/app"
	"pkgsilo/core/logger"
	"pkgsilo/core/router"
	"pkgsilo/core/telemetry"
	apikeyapi "pkgsilo/feature/apikey/api"
	groupapi "pkgsilo/feature/group/api"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/databasetest"
)

const (
	// apiKeyPepper is the secret key of the API key hashes in the tests' databases.
	apiKeyPepper = "test-pepper-of-at-least-32-characters"
	// apiKeyHeader is the header that holds a request's API key.
	apiKeyHeader = "X-API-Key" //nolint:gosec // The name of a header, not a credential.
)

// New returns a client for the application, wired as cmd/main.go serves it, on a new database with
// all migrations applied and the first start done. The client authenticates as its identity admin:
// use its Anonymous method for requests without a credential. The application logs to the test's
// output, which is shown for failed tests and with -v. New skips the test in short mode.
func New(t *testing.T) *apitest.Client {
	t.Helper()

	api, _, _ := newApp(t)

	return api
}

// NewWithDB is like New and also returns the application's database, for what the API can't set up.
func NewWithDB(t *testing.T) (*apitest.Client, *sqlx.DB) {
	t.Helper()

	api, _, db := newApp(t)

	return api, db
}

// NewWithMetrics is like New and also returns a client for the application's metrics listener, which
// serves the metrics that the application records, at telemetry.MetricsPath.
func NewWithMetrics(t *testing.T) (api, metrics *apitest.Client) {
	t.Helper()

	api, metrics, _ = newApp(t)

	return api, metrics
}

// NewInstance returns a client for another instance of the application on db, which NewWithDB returned
// together with api. The client authenticates like api. The instances share nothing but the database, like
// the instances of a deployment. Unlike in a deployment, they also share db's connection pool, whose
// metrics only the first instance records.
func NewInstance(t *testing.T, api *apitest.Client, db *sqlx.DB) *apitest.Client {
	t.Helper()

	return api.WithHandler(newHandler(t, db, newTelemetry(t)))
}

// newApp returns clients for the API and the metrics listener of the application on a new database with
// all migrations applied and the first start done, and that database. The API client authenticates as admin.
// As in cmd/main.go, the application and its database record their metrics with the same provider.
func newApp(t *testing.T) (api, metrics *apitest.Client, db *sqlx.DB) {
	t.Helper()

	tel := newTelemetry(t)
	db = databasetest.NewWithMeterProvider(t, tel.MeterProvider())

	key, err := app.Bootstrap(t.Context(), db, []byte(apiKeyPepper))
	require.NoError(t, err)

	h := newHandler(t, db, tel)

	return apitest.NewClient(t, h).WithHeader(apiKeyHeader, key), apitest.NewClient(t, tel.Handler()), db
}

// newTelemetry returns the enabled telemetry of an application instance, which is shut down at the end of
// the test, after the database has been closed.
func newTelemetry(t *testing.T) *telemetry.Telemetry {
	t.Helper()

	tel, err := telemetry.New(t.Context(), telemetry.Options{Enabled: true, ServiceVersion: "test"})
	require.NoError(t, err)
	t.Cleanup(func() {
		//nolint:usetesting // The test's context is already canceled when cleanups run.
		assert.NoError(t, tel.Shutdown(context.Background()))
	})

	return tel
}

// newHandler returns the application, wired as cmd/main.go serves it, on db, recording its metrics with
// tel. It logs to the test's output.
func newHandler(t *testing.T, db *sqlx.DB, tel *telemetry.Telemetry) http.Handler {
	t.Helper()

	return app.NewHandler(router.Options{}, db, []byte(apiKeyPepper),
		logger.New(t.Output(), logger.Options{Level: slog.LevelInfo}), tel.MeterProvider())
}

// NewIdentity creates an identity with the given username and an API key of its own through the client
// admin. It returns the identity and a client that authenticates as it, which only has the access that
// policies give an identity outside the administrators group.
func NewIdentity(admin *apitest.Client, username string) (identityapi.IdentityResponse, *apitest.Client) {
	const identitiesPath = "/api/v1/identities"

	created := apitest.DecodeResource[identityapi.IdentityResponse](
		admin.Post(identitiesPath, `{"username":"`+username+`","displayName":"`+username+`"}`), http.StatusCreated)
	key := apitest.DecodeResource[apikeyapi.CreatedKeyResponse](
		admin.Post(identitiesPath+"/"+created.ID.String()+"/api-keys", `{"name":"test"}`), http.StatusCreated)

	return created, admin.WithHeader(apiKeyHeader, key.Key.Value)
}

// AdministratorsGroup returns the ID of the group administrators, which the first start created with admin
// as its member and a policy that grants it everything. It finds the group by its name through the client
// admin.
func AdministratorsGroup(t *testing.T, admin *apitest.Client) uuid.UUID {
	t.Helper()

	groups := apitest.DecodeCollection[groupapi.GroupResponse](admin.Get("/api/v1/groups"), http.StatusOK).Data
	for _, group := range groups {
		if group.Name.Value == "administrators" {
			return group.ID
		}
	}
	require.FailNow(t, "no group named administrators on the first page")

	return uuid.Nil()
}
