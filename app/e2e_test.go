// End-to-end tests of what the app assembles across features: each test drives the whole app, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps and the
// results it expects.
package app_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/telemetry"
	"pkgsilo/test/apptest"
)

// TestMetrics checks that the metrics listener serves what the app records:
//
//  1. Scrape the metrics: 200 in the Prometheus text format, with target_info describing the service
//     pkgsilo and a Go runtime series.
func TestMetrics(t *testing.T) {
	t.Parallel()
	_, metrics := apptest.NewWithMetrics(t)

	// 1. Scrape.
	scrape := metrics.Get(telemetry.MetricsPath)
	scrape.AssertStatus(http.StatusOK)
	assert.Contains(t, scrape.Header().Get("Content-Type"), "text/plain")
	body := scrape.Body.String()
	assert.Regexp(t, `(?m)^target_info\{[^}]*service_name="pkgsilo"[^}]*\} 1$`, body)
	assert.Regexp(t, `(?m)^go_goroutine_count(_\w+)?\{`, body)
}
