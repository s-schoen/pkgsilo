// End-to-end tests of the health endpoint: each test drives the whole app through /api/v1, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test says what it requests and
// the result it expects.
package health_test

import (
	"net/http"
	"testing"

	"pkgsilo/test/apptest"
)

// TestHealth_DatabaseReachable gets the health with the database up: 200, healthy, with the database
// component healthy.
func TestHealth_DatabaseReachable(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	api.Get("/api/v1/health").AssertJSON(http.StatusOK, `{
		"status": "healthy",
		"service": "pkgsilo",
		"components": [{"name":"database","status":"healthy"}]
	}`)
}
