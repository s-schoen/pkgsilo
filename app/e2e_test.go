// End-to-end tests of what the app assembles across features: each test drives the whole app, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps and the
// results it expects.
package app_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/envelope"
	"pkgsilo/core/telemetry"
	identityapi "pkgsilo/feature/identity/api"
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
	identities := api.Get("/api/v1/identities").DecodeCollection[identityapi.IdentityResponse](http.StatusOK).Data
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
//  3. Find the spans of authentication and authorization: under the server span, one
//     pkgsilo.authn.authenticate with outcome success and method api_key, and one pkgsilo.authz.load.
//     The server span's enduser.id is the ID of the identity admin, which the listing holds.
//  4. Find the database spans: every one, among them the listing's SELECT on identities, descends from
//     the server span. The API key lookup descends from the authentication span, and the policy queries
//     from the authorization span. The queries of the first start ran outside a request, so they have
//     none.
func TestTracing(t *testing.T) {
	t.Parallel()
	api, spans := apptest.NewWithTracing(t)
	const sentTraceID, sentSpanID = "4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7"

	// 1. List.
	list := api.WithHeader("Traceparent", "00-"+sentTraceID+"-"+sentSpanID+"-01").Get("/api/v1/identities")
	identities := list.DecodeCollection[identityapi.IdentityResponse](http.StatusOK).Data

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

	// 3. Find the spans of authentication and authorization.
	authnSpan := onlySpan(t, spans.Spans(), "pkgsilo.authn.authenticate")
	assert.Equal(t, server.SpanContext().SpanID(), authnSpan.Parent().SpanID())
	authnAttrs := attribute.NewSet(authnSpan.Attributes()...)
	outcome, _ := authnAttrs.Value("pkgsilo.authn.outcome")
	assert.Equal(t, "success", outcome.AsString())
	method, _ := authnAttrs.Value("pkgsilo.authn.method")
	assert.Equal(t, "api_key", method.AsString())
	authzSpan := onlySpan(t, spans.Spans(), "pkgsilo.authz.load")
	assert.Equal(t, server.SpanContext().SpanID(), authzSpan.Parent().SpanID())
	require.Len(t, identities, 1)
	assert.Equal(t, envelope.Visible("admin"), identities[0].Username)
	enduserID, _ := attrs.Value("enduser.id")
	assert.Equal(t, identities[0].ID.String(), enduserID.AsString())

	// 4. Find the database spans.
	byID := map[trace.SpanID]sdktrace.ReadOnlySpan{}
	for _, span := range spans.Spans() {
		byID[span.SpanContext().SpanID()] = span
	}
	queries := map[sdktrace.ReadOnlySpan]string{}
	for _, span := range spans.Spans() {
		attrs := attribute.NewSet(span.Attributes()...)
		if !attrs.HasValue("db.system.name") {
			continue
		}
		query, _ := attrs.Value("db.query.text")
		queries[span] = query.AsString()
		assert.Truef(t, descendsFrom(span, server, byID), "%s %q doesn't descend from the server span",
			span.Name(), query.AsString())
	}
	assert.True(t, hasQueryUnder(queries, "FROM identities", server, byID),
		"no span of the query that lists the identities")
	assert.True(t, hasQueryUnder(queries, "FROM api_keys", authnSpan, byID),
		"no span of the API key lookup under the authentication span")
	assert.True(t, hasQueryUnder(queries, "FROM policy_revision", authzSpan, byID),
		"no span of the policy revision query under the authorization span")
}

const mcpPath = "/mcp"

// TestMCP checks what the app assembles at its MCP endpoint:
//
//  1. Send an MCP request without a key: 401 with the ApiKey challenge, and an MCP client without a key
//     can't connect.
//  2. List the tools as admin: exactly the tools that the features register.
//  3. Send a GET with the key: 405, since the endpoint is stateless and serves no event stream.
//  4. Call identity_whoami as admin: admin, as the API lists it.
func TestMCP(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	mcp := apptest.MCP(t, api)

	// 1. Without a key.
	resp := api.Anonymous().Post(mcpPath, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	resp.AssertProblem(envelope.UnauthorizedProblem.New(envelope.Occurrence{}))
	assert.Equal(t, []string{`ApiKey realm="pkgsilo"`}, resp.Header().Values("WWW-Authenticate"))
	err := apptest.MCP(t, api.Anonymous()).Connect()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unauthorized")

	// 2. List the tools.
	assert.Equal(t, []string{"identity_whoami"}, mcp.ToolNames())

	// 3. GET.
	get := api.Get(mcpPath)
	get.AssertStatus(http.StatusMethodNotAllowed)
	assert.Equal(t, http.MethodPost, get.Header().Get("Allow"))

	// 4. Who am I.
	admin := onlyIdentity(t, api.Get("/api/v1/identities").DecodeCollection[identityapi.IdentityResponse](http.StatusOK))
	assert.Equal(t, admin, mcp.Call("identity_whoami", "").Decode[identityapi.IdentityResponse]())
}

// TestMCP_Telemetry checks the telemetry of a tool call:
//
//  1. Call identity_whoami with tracing on: the span tools/call identity_whoami, a child of the server span
//     POST /mcp, with the tool's attributes.
//  2. Call identity_whoami with metrics on, and scrape the metrics: one tools/call of identity_whoami in
//     mcp_server_operation_duration_seconds, without an error, and the request in the HTTP server metrics
//     under the route /mcp.
func TestMCP_Telemetry(t *testing.T) {
	t.Parallel()

	// 1. Tracing.
	traced, spans := apptest.NewWithTracing(t)
	apptest.MCP(t, traced).Call("identity_whoami", "").Decode[identityapi.IdentityResponse]()
	call := onlySpan(t, spans.Spans(), "tools/call identity_whoami")
	byID := map[trace.SpanID]sdktrace.ReadOnlySpan{}
	for _, span := range spans.Spans() {
		byID[span.SpanContext().SpanID()] = span
	}
	server, ok := byID[call.Parent().SpanID()]
	require.True(t, ok, "the parent of the call's span ended")
	assert.Equal(t, "POST /mcp", server.Name())
	assert.Equal(t, trace.SpanKindServer, server.SpanKind())
	attrs := attribute.NewSet(call.Attributes()...)
	for key, want := range map[attribute.Key]string{
		"mcp.method.name": "tools/call", "gen_ai.tool.name": "identity_whoami", "gen_ai.operation.name": "execute_tool",
	} {
		got, _ := attrs.Value(key)
		assert.Equal(t, want, got.AsString(), key)
	}
	assert.False(t, attrs.HasValue("error.type"), "error.type")

	// 2. Metrics.
	measured, metrics := apptest.NewWithMetrics(t)
	apptest.MCP(t, measured).Call("identity_whoami", "").Decode[identityapi.IdentityResponse]()
	body := metrics.Get(telemetry.MetricsPath).Body.String()
	assert.Regexp(t, `(?m)^mcp_server_operation_duration_seconds_count\{gen_ai_operation_name="execute_tool",`+
		`gen_ai_tool_name="identity_whoami",mcp_method_name="tools/call",[^}]*\} 1$`, body)
	assert.NotRegexp(t, `(?m)^mcp_server_operation_duration_seconds_count\{[^}]*error_type=`, body)
	assert.Regexp(t, `(?m)^http_server_request_duration_seconds_count\{http_request_method="POST",`+
		`http_response_status_code="200",http_route="/mcp",[^}]*\} [1-9]\d*$`, body)
}

// onlyIdentity returns the only identity of a listing, failing the test if there are others.
func onlyIdentity(
	t *testing.T, listing envelope.Collection[identityapi.IdentityResponse],
) identityapi.IdentityResponse {
	t.Helper()
	require.Len(t, listing.Data, 1)

	return listing.Data[0]
}

// hasQueryUnder reports whether one of the database spans in queries, which maps each to its query text,
// descends from ancestor and runs a query that contains fragment.
func hasQueryUnder(
	queries map[sdktrace.ReadOnlySpan]string, fragment string, ancestor sdktrace.ReadOnlySpan,
	byID map[trace.SpanID]sdktrace.ReadOnlySpan,
) bool {
	for span, query := range queries {
		if strings.Contains(query, fragment) && descendsFrom(span, ancestor, byID) {
			return true
		}
	}

	return false
}

// onlySpan returns the span with the given name, failing the test if there's not exactly one.
func onlySpan(t *testing.T, spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	t.Helper()

	var found []sdktrace.ReadOnlySpan
	for _, span := range spans {
		if span.Name() == name {
			found = append(found, span)
		}
	}
	require.Len(t, found, 1, name)

	return found[0]
}

// descendsFrom reports whether ancestor is the parent of span, or of one of its ancestors in byID.
func descendsFrom(span, ancestor sdktrace.ReadOnlySpan, byID map[trace.SpanID]sdktrace.ReadOnlySpan) bool {
	for span.Parent().IsValid() {
		if span.Parent().SpanID() == ancestor.SpanContext().SpanID() {
			return true
		}
		parent, ok := byID[span.Parent().SpanID()]
		if !ok {
			return false
		}
		span = parent
	}

	return false
}
