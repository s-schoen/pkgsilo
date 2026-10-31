// Package apptest serves the whole application to end-to-end tests.
//
// It depends on every feature through app, so only the end-to-end tests at the root of a feature
// can import it, not the tests of the feature's layer packages.
package apptest

import (
	"log/slog"
	"testing"

	"pkgsilo/app"
	"pkgsilo/core/logger"
	"pkgsilo/core/router"
	"pkgsilo/test/apitest"
	"pkgsilo/test/databasetest"
)

// New returns a client for the application, wired as cmd/main.go serves it, on a new database with
// all migrations applied. The application logs to the test's output, which is shown for failed
// tests and with -v. New skips the test in short mode.
func New(t *testing.T) *apitest.Client {
	t.Helper()

	db := databasetest.New(t)
	log := logger.New(t.Output(), logger.Options{Level: slog.LevelInfo})

	return apitest.NewClient(t, app.NewHandler(router.Options{}, db, log))
}
