// Package apptest serves the whole application to end-to-end tests.
//
// It depends on every feature through app, so only the end-to-end tests at the root of a feature
// can import it, not the tests of the feature's layer packages.
package apptest

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"pkgsilo/app"
	"pkgsilo/core/logger"
	"pkgsilo/core/router"
	apikeyapi "pkgsilo/feature/apikey/api"
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

	api, _ := NewWithDB(t)

	return api
}

// NewWithDB is like New and also returns the application's database, for what the API can't set up.
func NewWithDB(t *testing.T) (*apitest.Client, *sqlx.DB) {
	t.Helper()

	db := databasetest.New(t)
	log := logger.New(t.Output(), logger.Options{Level: slog.LevelInfo})

	key, err := app.Bootstrap(t.Context(), db, []byte(apiKeyPepper))
	require.NoError(t, err)

	api := apitest.NewClient(t, app.NewHandler(router.Options{}, db, []byte(apiKeyPepper), log)).
		WithHeader(apiKeyHeader, key)

	return api, db
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

	return created, admin.WithHeader(apiKeyHeader, key.Key)
}
