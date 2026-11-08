// Package telemetry sets up the application's OpenTelemetry metrics and serves them to Prometheus.
//
// Instruments use the OpenTelemetry metric API. The provider's only reader is the OpenTelemetry
// Prometheus exporter, which registers with a registry of its own, never Prometheus' default one, so
// that two instances in one process, such as parallel tests, share nothing. Components that record
// metrics take the provider as a parameter: none of them reads the global one.
package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"uuid"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// ServiceName is the service.name of the resource, unless OTEL_SERVICE_NAME or OTEL_RESOURCE_ATTRIBUTES
// override it.
const ServiceName = "pkgsilo"

// MetricsPath is the path at which the handler serves the metrics.
const MetricsPath = "/metrics"

// Options configures the telemetry.
type Options struct {
	// Enabled turns metrics on. Without it, the provider is a no-op and the handler answers every
	// request with 404.
	Enabled bool
	// ServiceVersion is the service.version of the resource, the version of the build.
	ServiceVersion string
}

// Telemetry holds the metric provider of the application and the handler that serves its metrics.
type Telemetry struct {
	meterProvider metric.MeterProvider
	handler       http.Handler
	shutdown      func(context.Context) error
}

// New returns the telemetry that opts configures. When enabled, the resource describes the process as
// the service ServiceName in the version opts.ServiceVersion with a random service.instance.id, which
// OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES override, and the handler also serves the Go runtime and
// process metrics.
func New(ctx context.Context, opts Options) (*Telemetry, error) {
	if !opts.Enabled {
		return &Telemetry{
			meterProvider: noop.NewMeterProvider(),
			handler:       http.NewServeMux(),
			shutdown:      func(context.Context) error { return nil },
		}, nil
	}

	res, err := newResource(ctx, opts.ServiceVersion)
	if err != nil {
		return nil, err
	}

	registry := prometheus.NewRegistry()
	if err := registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		return nil, fmt.Errorf("register process collector: %w", err)
	}
	exporter, err := otelprometheus.New(otelprometheus.WithRegisterer(registry))
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res), sdkmetric.WithReader(exporter))

	if err := runtime.Start(runtime.WithMeterProvider(provider)); err != nil {
		return nil, fmt.Errorf("start runtime metrics: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET "+MetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	return &Telemetry{
		meterProvider: provider,
		handler:       mux,
		shutdown:      provider.Shutdown,
	}, nil
}

// MeterProvider returns the provider that components record their metrics with.
func (t *Telemetry) MeterProvider() metric.MeterProvider {
	return t.meterProvider
}

// Handler returns the handler of the metrics listener: it serves the metrics in the Prometheus
// exposition formats at GET MetricsPath, and nothing else.
func (t *Telemetry) Handler() http.Handler {
	return t.handler
}

// Shutdown shuts the provider down, after which its instruments record nothing. Call it once the
// components that record metrics have stopped.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if err := t.shutdown(ctx); err != nil {
		return fmt.Errorf("shut down meter provider: %w", err)
	}

	return nil
}

// newResource returns the resource that describes this process. The environment overrides the defaults.
func newResource(ctx context.Context, version string) (*resource.Resource, error) {
	res, err := resource.New(ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(
			semconv.ServiceName(ServiceName),
			semconv.ServiceVersion(version),
			semconv.ServiceInstanceID(uuid.New().String()),
		),
		// Detected last, so that its attributes override the ones above.
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	return res, nil
}
