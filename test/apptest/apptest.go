// Package apptest serves the whole application to end-to-end tests.
//
// It depends on every feature through app, so only the end-to-end tests at the root of a feature
// can import it, not the tests of the feature's layer packages.
package apptest

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"pkgsilo/app"
	"pkgsilo/core/logger"
	"pkgsilo/core/router"
	"pkgsilo/test/apitest"
	"pkgsilo/test/databasetest"
)

// apiKeyPepper is the secret key of the API key hashes in the tests' databases.
const apiKeyPepper = "test-pepper-of-at-least-32-characters"

// New returns a client for the application, wired as cmd/main.go serves it, on a new database with
// all migrations applied and the first start done. The client authenticates as its identity admin:
// use its Anonymous method for requests without a credential. The application logs to the test's
// output, which is shown for failed tests and with -v. New skips the test in short mode.
func New(t *testing.T) *apitest.Client {
	t.Helper()

	db := databasetest.New(t)
	log := logger.New(t.Output(), logger.Options{Level: slog.LevelInfo})

	key, err := app.Bootstrap(t.Context(), db, []byte(apiKeyPepper))
	require.NoError(t, err)

	return apitest.NewClient(t, app.NewHandler(router.Options{}, db, []byte(apiKeyPepper), log)).
		WithHeader("X-API-Key", key)
}
