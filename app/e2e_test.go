// End-to-end tests of what the app assembles across features: each test drives the whole app, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps and the
// results it expects.
package app_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/telemetry"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/apptest"
)

// TestMetrics checks that the metrics listener serves what the app records:
//
//  1. List the identities: 200.
//  2. Get the first one: 200.
//  3. Check the health, without and with a trailing slash: 200 each.
//  4. Get the first identity without a key: 401.
//  5. List the identities with an unknown key: 401.
//  6. Scrape the metrics: 200 in the Prometheus text format, with target_info describing the service
//     pkgsilo, a Go runtime series, the request durations of steps 1 and 2 under their route
//     patterns, and of steps 4 and 5 under the same patterns, without the host they were sent to,
//     the connections of the database pool, the durations of the queries, the authentications of
//     steps 1 and 2 as successes, of step 4 as missing and of step 5 as rejected for an unknown key,
//     and the allowed authorization decisions. Step 3 isn't measured.
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
	api.Anonymous().Get("/api/v1/health/").AssertStatus(http.StatusOK)

	// 4. Get without a key.
	api.Anonymous().Get("/api/v1/identities/" + identities[0].ID.String()).AssertStatus(http.StatusUnauthorized)

	// 5. List with an unknown key.
	api.Anonymous().WithHeader("X-Api-Key", "psk_unknown").Get("/api/v1/identities").
		AssertStatus(http.StatusUnauthorized)

	// 6. Scrape.
	scrape := metrics.Get(telemetry.MetricsPath)
	scrape.AssertStatus(http.StatusOK)
	assert.Contains(t, scrape.Header().Get("Content-Type"), "text/plain")
	body := scrape.Body.String()
	assert.Regexp(t, `(?m)^target_info\{[^}]*service_name="pkgsilo"[^}]*\} 1$`, body)
	assert.Regexp(t, `(?m)^go_goroutine_count(_\w+)?\{`, body)
	for _, route := range []string{`/api/v1/identities`, `/api/v1/identities/\{id\}`} {
		for _, status := range []string{"200", "401"} {
			assert.Regexp(t, `(?m)^http_server_request_duration_seconds_count\{http_request_method="GET",`+
				`http_response_status_code="`+status+`",http_route="`+route+`",[^}]*\} 1$`, body)
		}
	}
	assert.NotRegexp(t, `(?m)^http_server_\w+\{[^}]*(server_address|server_port)=`, body)
	assert.NotContains(t, body, "/api/v1/health")
	assert.Regexp(t, `(?m)^db_client_connection_count\{[^}]*db_client_connection_state="used"[^}]*\} \d+$`, body)
	assert.Regexp(t, `(?m)^db_client_operation_duration_seconds_count\{[^}]*db_system_name="postgresql",`+
		`[^}]*pgx_operation_type="query"[^}]*\} [1-9]\d*$`, body)
	for _, attempts := range []string{
		`pkgsilo_authn_method="api_key",pkgsilo_authn_outcome="success"\} 2`,
		`pkgsilo_authn_outcome="missing"\} 1`,
		`pkgsilo_authn_method="api_key",pkgsilo_authn_outcome="rejected",pkgsilo_authn_reason="unknown"\} 1`,
	} {
		assert.Regexp(t, `(?m)^pkgsilo_authn_attempts_total\{[^}]*`+attempts+`$`, body)
	}
	assert.Regexp(t, `(?m)^pkgsilo_authz_decisions_total\{[^}]*pkgsilo_authz_decision="allow"\} [1-9]\d*$`, body)
}

// TestTracing checks that the app creates the spans of a request with the tracer provider it's given:
//
//  1. List the identities with a traceparent header: 200.
//  2. Find the spans: one server span named GET /api/v1/identities, which holds the request ID of the
//     response. The test client isn't a trusted proxy, so the span starts a new trace instead of
//     continuing the sent one, and links to the sent span.
func TestTracing(t *testing.T) {
	t.Parallel()
	api, spans := apptest.NewWithTracing(t)
	const sentTraceID, sentSpanID = "4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7"

	// 1. List.
	list := api.WithHeader("Traceparent", "00-"+sentTraceID+"-"+sentSpanID+"-01").Get("/api/v1/identities")
	list.AssertStatus(http.StatusOK)

	// 2. Find the spans.
	var servers []sdktrace.ReadOnlySpan
	for _, span := range spans.Spans() {
		if span.SpanKind() == trace.SpanKindServer {
			servers = append(servers, span)
		}
	}
	require.Len(t, servers, 1)
	server := servers[0]
	assert.Equal(t, "GET /api/v1/identities", server.Name())
	attrs := attribute.NewSet(server.Attributes()...)
	requestID, _ := attrs.Value("pkgsilo.request.id")
	assert.Equal(t, list.Header().Get("X-Request-ID"), requestID.AsString())
	assert.NotEqual(t, sentTraceID, server.SpanContext().TraceID().String())
	assert.False(t, server.Parent().IsValid(), "the server span has no parent")
	require.Len(t, server.Links(), 1)
	assert.Equal(t, sentTraceID, server.Links()[0].SpanContext.TraceID().String())
	assert.Equal(t, sentSpanID, server.Links()[0].SpanContext.SpanID().String())
}
