// Package telemetrytest records the metrics of a test in memory and reads them back.
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

	"pkgsilo/core/telemetry"
)

// Recorder is a metric provider with the views of telemetry.New whose only reader is an SDK ManualReader,
// so that a test reads exactly what its instruments recorded, shared with no other test.
type Recorder struct {
	provider *sdkmetric.MeterProvider
	reader   *sdkmetric.ManualReader
}

// New returns a Recorder, shut down at the end of the test.
func New(t *testing.T) *Recorder {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(telemetry.Views()...))
	t.Cleanup(func() {
		//nolint:usetesting // The test's context is already canceled when cleanups run.
		assert.NoError(t, provider.Shutdown(context.Background()))
	})

	return &Recorder{provider: provider, reader: reader}
}

// MeterProvider returns the provider to pass to the component under test.
func (r *Recorder) MeterProvider() metric.MeterProvider {
	return r.provider
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
func HistogramPoints[N int64 | float64](tb testing.TB, r *Recorder, name string) []metricdata.HistogramDataPoint[N] {
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
func SumPoints[N int64 | float64](tb testing.TB, r *Recorder, name string) []metricdata.DataPoint[N] {
	tb.Helper()

	m, ok := r.Metric(tb, name)
	if !ok {
		return nil
	}
	sum, ok := m.Data.(metricdata.Sum[N])
	require.Truef(tb, ok, "metric %s is a %T, no sum of %T", name, m.Data, N(0))

	return sum.DataPoints
}
