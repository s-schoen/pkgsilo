package telemetry

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// newEnabled returns telemetry with metrics on and tracing off in version 1.2.3, shut down at the end
// of the test.
func newEnabled(t *testing.T) *Telemetry {
	t.Helper()

	return newTelemetry(t, Options{Metrics: true, ServiceVersion: "1.2.3"})
}

// newTelemetry returns the telemetry that opts configures, shut down at the end of the test.
func newTelemetry(t *testing.T, opts Options) *Telemetry {
	t.Helper()

	tel, err := New(t.Context(), opts)
	require.NoError(t, err)
	t.Cleanup(func() {
		//nolint:usetesting // The test's context is already canceled when cleanups run.
		assert.NoError(t, tel.Shutdown(context.Background()))
	})

	return tel
}

// collector is an OTLP/HTTP trace receiver that keeps the spans it receives.
type collector struct {
	// url is the base URL to set as OTEL_EXPORTER_OTLP_ENDPOINT.
	url   string
	mu    sync.Mutex
	spans []*tracepb.ResourceSpans
}

// newCollector starts a collector for the rest of the test. It clears the variables that would send the
// spans elsewhere or drop them, so the test can't run in parallel.
func newCollector(t *testing.T) *collector {
	t.Helper()

	c := &collector{}
	server := httptest.NewServer(http.HandlerFunc(c.serveHTTP))
	t.Cleanup(server.Close)
	c.url = server.URL
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_TRACES_SAMPLER", "")

	return c
}

func (c *collector) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/traces" {
		http.NotFound(w, r)
		return
	}

	var body io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body = gz
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req coltracepb.ExportTraceServiceRequest
	if err := proto.Unmarshal(raw, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	c.spans = append(c.spans, req.GetResourceSpans()...)
	c.mu.Unlock()

	resp, err := proto.Marshal(&coltracepb.ExportTraceServiceResponse{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	_, _ = w.Write(resp)
}

// received returns the spans that the collector has received, by their names, each with the attributes
// of its resource.
func (c *collector) received() map[string]map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()

	spans := map[string]map[string]string{}
	for _, rs := range c.spans {
		resource := map[string]string{}
		for _, attr := range rs.GetResource().GetAttributes() {
			resource[attr.GetKey()] = attr.GetValue().GetStringValue()
		}
		for _, ss := range rs.GetScopeSpans() {
			for _, span := range ss.GetSpans() {
				spans[span.GetName()] = resource
			}
		}
	}

	return spans
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

	assert.IsType(t, metricnoop.MeterProvider{}, tel.MeterProvider())
	assert.IsType(t, tracenoop.TracerProvider{}, tel.TracerProvider())
	assertNoMetrics(t, tel)
	assert.NoError(t, tel.Shutdown(t.Context()))
}

func TestNew_MetricsWithoutTracing(t *testing.T) {
	t.Parallel()
	tel := newEnabled(t)

	assert.IsType(t, tracenoop.TracerProvider{}, tel.TracerProvider())
	counter, err := tel.MeterProvider().Meter("pkgsilo/test").Int64Counter("test.requests")
	require.NoError(t, err)
	counter.Add(t.Context(), 1)
	assert.Regexp(t, `(?m)^test_requests_total\{[^}]*\} 1$`, scrape(t, tel))
}

func TestNew_TracingExportsOnShutdown(t *testing.T) {
	c := newCollector(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", c.url)
	tel, err := New(t.Context(), Options{Tracing: true, ServiceVersion: "1.2.3"})
	require.NoError(t, err)

	_, span := tel.TracerProvider().Tracer("pkgsilo/test").Start(t.Context(), "test.operation")
	span.End()
	require.NoError(t, tel.Shutdown(t.Context()))

	received := c.received()
	require.Contains(t, received, "test.operation")
	resource := received["test.operation"]
	assert.Equal(t, "pkgsilo", resource[string(semconv.ServiceNameKey)])
	assert.Equal(t, "1.2.3", resource[string(semconv.ServiceVersionKey)])
	assert.Regexp(t, `^[0-9a-f-]{36}$`, resource[string(semconv.ServiceInstanceIDKey)])
}

func TestNew_TracingWithoutMetrics(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", newCollector(t).url)
	tel := newTelemetry(t, Options{Tracing: true})

	assert.IsType(t, &sdktrace.TracerProvider{}, tel.TracerProvider())
	assert.IsType(t, metricnoop.MeterProvider{}, tel.MeterProvider())
	assertNoMetrics(t, tel)
}

func TestNew_SpansAndMetricsShareResource(t *testing.T) {
	c := newCollector(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", c.url)
	tel, err := New(t.Context(), Options{Metrics: true, Tracing: true})
	require.NoError(t, err)

	_, span := tel.TracerProvider().Tracer("pkgsilo/test").Start(t.Context(), "test.operation")
	span.End()
	info := targetInfo(t, scrape(t, tel))
	require.NoError(t, tel.Shutdown(t.Context()))

	received := c.received()
	require.Contains(t, received, "test.operation")
	instanceID := received["test.operation"][string(semconv.ServiceInstanceIDKey)]
	require.NotEmpty(t, instanceID)
	assert.Contains(t, info, `service_instance_id="`+instanceID+`"`)
}

// assertNoMetrics asserts that the handler of tel serves no metrics.
func assertNoMetrics(t *testing.T, tel *Telemetry) {
	t.Helper()

	rec := httptest.NewRecorder()
	tel.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, MetricsPath, nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// targetInfo returns the line of the target_info series in the scraped body.
func targetInfo(t *testing.T, body string) string {
	t.Helper()

	line := regexp.MustCompile(`(?m)^target_info\{.*\} 1$`).FindString(body)
	require.NotEmpty(t, line, "no target_info series in:\n%s", body)

	return line
}
