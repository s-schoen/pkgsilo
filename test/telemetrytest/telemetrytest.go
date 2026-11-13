// Package telemetrytest records the metrics and spans of a test in memory and reads them back.
//
// It must import nothing from pkgsilo but core/telemetry, so that the internal tests of core packages
// such as core/router can use it without an import cycle.
package telemetrytest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/telemetry"
)

// Recorder holds a meter provider and a tracer provider that record in memory, shared with no other test.
// The meter provider has the views of telemetry.New, and its only reader is an SDK ManualReader, so that
// a test reads exactly what its instruments recorded. The tracer provider samples every span whose parent
// is sampled or that has none, whatever OTEL_TRACES_SAMPLER says, and keeps the spans once they end.
type Recorder struct {
	meterProvider  *sdkmetric.MeterProvider
	reader         *sdkmetric.ManualReader
	tracerProvider *sdktrace.TracerProvider
	spans          *tracetest.SpanRecorder
}

// New returns a Recorder, shut down at the end of the test.
func New(t *testing.T) *Recorder {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(telemetry.Views()...))
	spans := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())), sdktrace.WithSpanProcessor(spans))
	t.Cleanup(func() {
		//nolint:usetesting // The test's context is already canceled when cleanups run.
		ctx := context.Background()
		assert.NoError(t, tracerProvider.Shutdown(ctx))
		assert.NoError(t, meterProvider.Shutdown(ctx))
	})

	return &Recorder{meterProvider: meterProvider, reader: reader, tracerProvider: tracerProvider, spans: spans}
}

// MeterProvider returns the meter provider to pass to the component under test.
func (r *Recorder) MeterProvider() metric.MeterProvider {
	return r.meterProvider
}

// TracerProvider returns the tracer provider to pass to the component under test.
func (r *Recorder) TracerProvider() trace.TracerProvider {
	return r.tracerProvider
}

// Spans returns the spans that have ended so far, in the order in which they ended.
func (r *Recorder) Spans() []sdktrace.ReadOnlySpan {
	return r.spans.Ended()
}

// Metric returns the metric with the given name as the provider collects it now, from whichever scope
// recorded it. It reports false when no instrument with the name has recorded anything.
func (r *Recorder) Metric(tb testing.TB, name string) (metricdata.Metrics, bool) {
	tb.Helper()

	var rm metricdata.ResourceMetrics
	require.NoError(tb, r.reader.Collect(tb.Context(), &rm))
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name == name {
				return m, true
			}
		}
	}

	return metricdata.Metrics{}, false
}

// HistogramPoints returns the data points of the histogram with the given name, one per attribute set, or
// nil when it has recorded nothing. It fails the test if the metric is no histogram of N.
func (r *Recorder) HistogramPoints[N int64 | float64](tb testing.TB, name string) []metricdata.HistogramDataPoint[N] {
	tb.Helper()

	m, ok := r.Metric(tb, name)
	if !ok {
		return nil
	}
	histogram, ok := m.Data.(metricdata.Histogram[N])
	require.Truef(tb, ok, "metric %s is a %T, no histogram of %T", name, m.Data, N(0))

	return histogram.DataPoints
}

// SumPoints returns the data points of the counter or up-down counter with the given name, synchronous or
// observable, one per attribute set, or nil when it has recorded nothing. It fails the test if the metric
// is no sum of N.
func (r *Recorder) SumPoints[N int64 | float64](tb testing.TB, name string) []metricdata.DataPoint[N] {
	tb.Helper()

	m, ok := r.Metric(tb, name)
	if !ok {
		return nil
	}
	sum, ok := m.Data.(metricdata.Sum[N])
	require.Truef(tb, ok, "metric %s is a %T, no sum of %T", name, m.Data, N(0))

	return sum.DataPoints
}
