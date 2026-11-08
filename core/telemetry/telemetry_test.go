package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// newEnabled returns enabled telemetry in version 1.2.3, shut down at the end of the test.
func newEnabled(t *testing.T) *Telemetry {
	t.Helper()

	tel, err := New(t.Context(), Options{Enabled: true, ServiceVersion: "1.2.3"})
	require.NoError(t, err)
	t.Cleanup(func() {
		//nolint:usetesting // The test's context is already canceled when cleanups run.
		assert.NoError(t, tel.Shutdown(context.Background()))
	})

	return tel
}

// scrape returns the metrics that the handler serves, in the Prometheus text format.
func scrape(t *testing.T, tel *Telemetry) string {
	t.Helper()

	rec := httptest.NewRecorder()
	tel.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, MetricsPath, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/plain")

	return rec.Body.String()
}

func TestNew_ServesInstruments(t *testing.T) {
	t.Parallel()
	tel := newEnabled(t)

	counter, err := tel.MeterProvider().Meter("pkgsilo/test").Int64Counter("test.requests", metric.WithUnit("{request}"))
	require.NoError(t, err)
	counter.Add(t.Context(), 3)

	body := scrape(t, tel)

	assert.Regexp(t, `(?m)^test_requests_total\{otel_scope_name="pkgsilo/test",[^}]*\} 3$`, body)
}

func TestNew_UnitSuffix(t *testing.T) {
	t.Parallel()
	tel := newEnabled(t)

	histogram, err := tel.MeterProvider().Meter("pkgsilo/test").Float64Histogram("test.duration", metric.WithUnit("s"))
	require.NoError(t, err)
	histogram.Record(t.Context(), 0.5)

	assert.Regexp(t, `(?m)^test_duration_seconds_count\{[^}]*\} 1$`, scrape(t, tel))
}

func TestNew_HTTPServerMetricsDropServerAddress(t *testing.T) {
	t.Parallel()
	tel := newEnabled(t)
	attrs := metric.WithAttributes(
		semconv.ServerAddress("attacker.example"), semconv.ServerPort(8443), semconv.HTTPRoute("/items"))

	meter := tel.MeterProvider().Meter("pkgsilo/test")
	server, err := meter.Float64Histogram("http.server.request.duration", metric.WithUnit("s"))
	require.NoError(t, err)
	server.Record(t.Context(), 0.5, attrs)
	client, err := meter.Float64Histogram("http.client.request.duration", metric.WithUnit("s"))
	require.NoError(t, err)
	client.Record(t.Context(), 0.5, attrs)

	body := scrape(t, tel)

	assert.Regexp(t, `(?m)^http_server_request_duration_seconds_count\{http_route="/items",[^}]*\} 1$`, body)
	assert.NotRegexp(t, `(?m)^http_server_request_duration_seconds_count\{[^}]*server_`, body)
	assert.Regexp(t,
		`(?m)^http_client_request_duration_seconds_count\{[^}]*server_address="attacker.example",server_port="8443"\} 1$`,
		body)
}

func TestNew_TargetInfo(t *testing.T) {
	t.Parallel()
	tel := newEnabled(t)

	info := targetInfo(t, scrape(t, tel))

	assert.Contains(t, info, `service_name="pkgsilo"`)
	assert.Contains(t, info, `service_version="1.2.3"`)
	assert.Regexp(t, `service_instance_id="[0-9a-f-]{36}"`, info)
}

func TestNew_InstancesDiffer(t *testing.T) {
	t.Parallel()

	instanceID := regexp.MustCompile(`service_instance_id="([^"]+)"`)
	first := instanceID.FindStringSubmatch(targetInfo(t, scrape(t, newEnabled(t))))
	second := instanceID.FindStringSubmatch(targetInfo(t, scrape(t, newEnabled(t))))

	require.Len(t, first, 2)
	require.Len(t, second, 2)
	assert.NotEqual(t, first[1], second[1])
}

func TestNew_ResourceFromEnv(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "pkgsilo-eu")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.version=4.5.6,deployment.environment.name=staging")
	tel := newEnabled(t)

	info := targetInfo(t, scrape(t, tel))

	assert.Contains(t, info, `service_name="pkgsilo-eu"`)
	assert.Contains(t, info, `service_version="4.5.6"`)
	assert.Contains(t, info, `deployment_environment_name="staging"`)
}

func TestNew_RuntimeAndProcessMetrics(t *testing.T) {
	t.Parallel()
	tel := newEnabled(t)

	body := scrape(t, tel)

	assert.Regexp(t, `(?m)^go_goroutine_count(_\w+)?\{`, body)
	assert.Regexp(t, `(?m)^process_cpu_seconds_total `, body)
}

func TestNew_ServesOnlyMetricsPath(t *testing.T) {
	t.Parallel()
	tel := newEnabled(t)

	for _, req := range []*http.Request{
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil),
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/health", nil),
		httptest.NewRequestWithContext(t.Context(), http.MethodPost, MetricsPath, nil),
	} {
		rec := httptest.NewRecorder()
		tel.Handler().ServeHTTP(rec, req)
		assert.NotEqual(t, http.StatusOK, rec.Code, "%s %s", req.Method, req.URL.Path)
	}
}

func TestNew_Disabled(t *testing.T) {
	t.Parallel()

	tel, err := New(t.Context(), Options{})
	require.NoError(t, err)

	assert.IsType(t, noop.MeterProvider{}, tel.MeterProvider())
	rec := httptest.NewRecorder()
	tel.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, MetricsPath, nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NoError(t, tel.Shutdown(t.Context()))
}

// targetInfo returns the line of the target_info series in the scraped body.
func targetInfo(t *testing.T, body string) string {
	t.Helper()

	line := regexp.MustCompile(`(?m)^target_info\{.*\} 1$`).FindString(body)
	require.NotEmpty(t, line, "no target_info series in:\n%s", body)

	return line
}
