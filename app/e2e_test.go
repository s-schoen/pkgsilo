// End-to-end tests of what the app assembles across features: each test drives the whole app, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps and the
// results it expects.
package app_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/telemetry"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/apptest"
)

// TestMetrics checks that the metrics listener serves what the app records:
//
//  1. List the identities: 200.
//  2. Get the first one: 200.
//  3. Check the health: 200.
//  4. Scrape the metrics: 200 in the Prometheus text format, with target_info describing the service
//     pkgsilo, a Go runtime series, the request durations of steps 1 and 2 under their route
//     patterns, without the host they were sent to, the connections of the database pool and the
//     durations of the queries. Step 3 isn't measured.
func TestMetrics(t *testing.T) {
	t.Parallel()
	api, metrics := apptest.NewWithMetrics(t)

	// 1. List.
	identities := apitest.DecodeCollection[identityapi.IdentityResponse](
		api.Get("/api/v1/identities"), http.StatusOK).Data
	require.NotEmpty(t, identities)

	// 2. Get.
	api.Get("/api/v1/identities/" + identities[0].ID.String()).AssertStatus(http.StatusOK)

	// 3. Check the health.
	api.Anonymous().Get("/api/v1/health").AssertStatus(http.StatusOK)

	// 4. Scrape.
	scrape := metrics.Get(telemetry.MetricsPath)
	scrape.AssertStatus(http.StatusOK)
	assert.Contains(t, scrape.Header().Get("Content-Type"), "text/plain")
	body := scrape.Body.String()
	assert.Regexp(t, `(?m)^target_info\{[^}]*service_name="pkgsilo"[^}]*\} 1$`, body)
	assert.Regexp(t, `(?m)^go_goroutine_count(_\w+)?\{`, body)
	for _, route := range []string{`/api/v1/identities`, `/api/v1/identities/\{id\}`} {
		assert.Regexp(t, `(?m)^http_server_request_duration_seconds_count\{http_request_method="GET",`+
			`http_response_status_code="200",http_route="`+route+`",[^}]*\} 1$`, body)
	}
	assert.NotRegexp(t, `(?m)^http_server_\w+\{[^}]*(server_address|server_port)=`, body)
	assert.NotContains(t, body, "/api/v1/health")
	assert.Regexp(t, `(?m)^db_client_connection_count\{[^}]*db_client_connection_state="used"[^}]*\} \d+$`, body)
	assert.Regexp(t, `(?m)^db_client_operation_duration_seconds_count\{[^}]*db_system_name="postgresql",`+
		`[^}]*pgx_operation_type="query"[^}]*\} [1-9]\d*$`, body)
}
