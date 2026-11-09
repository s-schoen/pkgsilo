// Package telemetry sets up the application's OpenTelemetry metrics, which it serves to Prometheus, and
// its traces, which it exports over OTLP/HTTP.
//
// Instruments use the OpenTelemetry metric API. The meter provider's only reader is the OpenTelemetry
// Prometheus exporter, which registers with a registry of its own, never Prometheus' default one, so
// that two instances in one process, such as parallel tests, share nothing.
//
// Spans go through a batch span processor to the OTLP/HTTP exporter. The exporter and the SDK read
// their configuration from the standard OTEL_* variables, such as OTEL_EXPORTER_OTLP_ENDPOINT,
// OTEL_EXPORTER_OTLP_HEADERS, OTEL_TRACES_SAMPLER and OTEL_BSP_*, so New sets none of it in code. The
// exporter connects only when it exports, so an unreachable collector doesn't fail New.
//
// Components that record metrics or create spans take the providers as parameters: none of them reads
// the global ones.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"uuid"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// ServiceName is the service.name of the resource, unless OTEL_SERVICE_NAME or OTEL_RESOURCE_ATTRIBUTES
// override it.
const ServiceName = "pkgsilo"

// MetricsPath is the path at which the handler serves the metrics.
const MetricsPath = "/metrics"

// Options configures the telemetry.
type Options struct {
	// Metrics turns metrics on. Without it, the meter provider is a no-op and the handler answers every
	// request with 404.
	Metrics bool
	// Tracing turns tracing on. Without it, the tracer provider is a no-op.
	Tracing bool
	// ServiceVersion is the service.version of the resource, the version of the build.
	ServiceVersion string
}

// Telemetry holds the meter and tracer providers of the application and the handler that serves its
// metrics.
type Telemetry struct {
	meterProvider  metric.MeterProvider
	tracerProvider trace.TracerProvider
	handler        http.Handler
	shutdownMeter  func(context.Context) error
	shutdownTracer func(context.Context) error
}

// New returns the telemetry that opts configures. The metrics and the spans share one resource, which
// describes the process as the service ServiceName in the version opts.ServiceVersion with a random
// service.instance.id, and which OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES override. With metrics
// on, the handler also serves the Go runtime and process metrics.
func New(ctx context.Context, opts Options) (*Telemetry, error) {
	tel := &Telemetry{
		meterProvider:  metricnoop.NewMeterProvider(),
		tracerProvider: tracenoop.NewTracerProvider(),
		handler:        http.NewServeMux(),
		shutdownMeter:  noShutdown,
		shutdownTracer: noShutdown,
	}
	if !opts.Metrics && !opts.Tracing {
		return tel, nil
	}

	res, err := newResource(ctx, opts.ServiceVersion)
	if err != nil {
		return nil, err
	}

	// Tracing first: its provider shuts down completely if the metrics then fail, while the runtime
	// metrics, once started, can't be stopped.
	if opts.Tracing {
		provider, err := newTracerProvider(ctx, res)
		if err != nil {
			return nil, err
		}
		tel.tracerProvider = provider
		tel.shutdownTracer = provider.Shutdown
	}

	if opts.Metrics {
		provider, handler, err := newMeterProvider(ctx, res)
		if err != nil {
			// The error of the metrics is the one that matters, and no span has been created yet.
			_ = tel.shutdownTracer(context.WithoutCancel(ctx))
			return nil, err
		}
		tel.meterProvider = provider
		tel.handler = handler
		tel.shutdownMeter = provider.Shutdown
	}

	return tel, nil
}

// newTracerProvider returns a tracer provider on res that exports its spans in batches over OTLP/HTTP.
// The exporter and the provider take their endpoint, sampler and batch options from the environment.
func newTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create otlp trace exporter: %w", err)
	}

	return sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithBatcher(exporter)), nil
}

// newMeterProvider returns a meter provider on res, with the Go runtime metrics started, and the handler
// that serves its metrics together with the process metrics.
func newMeterProvider(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, http.Handler, error) {
	registry := prometheus.NewRegistry()
	if err := registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		return nil, nil, fmt.Errorf("register process collector: %w", err)
	}
	exporter, err := otelprometheus.New(otelprometheus.WithRegisterer(registry))
	if err != nil {
		return nil, nil, fmt.Errorf("create prometheus exporter: %w", err)
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res), sdkmetric.WithReader(exporter), sdkmetric.WithView(Views()...))

	if err := runtime.Start(runtime.WithMeterProvider(provider)); err != nil {
		// The error of the start is the one that matters, and the provider has recorded nothing yet.
		_ = provider.Shutdown(context.WithoutCancel(ctx))
		return nil, nil, fmt.Errorf("start runtime metrics: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET "+MetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	return provider, mux, nil
}

// noShutdown is the shutdown of a provider that is turned off.
func noShutdown(context.Context) error {
	return nil
}

// MeterProvider returns the provider that components record their metrics with.
func (t *Telemetry) MeterProvider() metric.MeterProvider {
	return t.meterProvider
}

// TracerProvider returns the provider that components create their spans with.
func (t *Telemetry) TracerProvider() trace.TracerProvider {
	return t.tracerProvider
}

// Handler returns the handler of the metrics listener: it serves the metrics in the Prometheus
// exposition formats at GET MetricsPath, and nothing else.
func (t *Telemetry) Handler() http.Handler {
	return t.handler
}

// Shutdown exports the spans that are still buffered and shuts both providers down, after which their
// instruments record nothing and their spans are dropped. Call it once the components that record
// metrics and create spans have stopped. It shuts down both even if one fails, and returns both errors.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	if err := t.shutdownTracer(ctx); err != nil {
		errs = append(errs, fmt.Errorf("shut down tracer provider: %w", err))
	}
	if err := t.shutdownMeter(ctx); err != nil {
		errs = append(errs, fmt.Errorf("shut down meter provider: %w", err))
	}

	return errors.Join(errs...)
}

// Views returns the views that the provider applies to what instruments record. They drop server.address
// and server.port from the HTTP server metrics: both come from the Host header that the client sends, so
// they can take any value, and semconv makes them opt-in on these metrics.
func Views() []sdkmetric.View {
	return []sdkmetric.View{
		sdkmetric.NewView(
			sdkmetric.Instrument{Name: "http.server.*"},
			sdkmetric.Stream{AttributeFilter: attribute.NewDenyKeysFilter(semconv.ServerAddressKey, semconv.ServerPortKey)},
		),
	}
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
