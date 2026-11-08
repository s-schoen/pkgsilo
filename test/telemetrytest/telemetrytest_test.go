package telemetrytest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"pkgsilo/test/tbtest"
)

func TestHistogramPoints(t *testing.T) {
	t.Parallel()
	rec := New(t)
	histogram, err := rec.MeterProvider().Meter("pkgsilo/test").Float64Histogram("test.duration")
	require.NoError(t, err)
	histogram.Record(t.Context(), 0.5, metric.WithAttributes(attribute.String("a", "1")))
	histogram.Record(t.Context(), 1.5, metric.WithAttributes(attribute.String("a", "1")))
	histogram.Record(t.Context(), 1, metric.WithAttributes(attribute.String("a", "2")))

	points := HistogramPoints[float64](t, rec, "test.duration")

	require.Len(t, points, 2)
	byA := map[string]uint64{}
	for _, p := range points {
		a, _ := p.Attributes.Value("a")
		byA[a.AsString()] = p.Count
	}
	assert.Equal(t, map[string]uint64{"1": 2, "2": 1}, byA)
}

func TestHistogramPoints_NothingRecorded(t *testing.T) {
	t.Parallel()
	rec := New(t)
	_, err := rec.MeterProvider().Meter("pkgsilo/test").Float64Histogram("test.duration")
	require.NoError(t, err)

	assert.Nil(t, HistogramPoints[float64](t, rec, "test.duration"))
}

func TestHistogramPoints_WrongType(t *testing.T) {
	t.Parallel()
	rec := New(t)
	histogram, err := rec.MeterProvider().Meter("pkgsilo/test").Int64Histogram("test.size")
	require.NoError(t, err)
	histogram.Record(t.Context(), 1)

	failed := tbtest.Run(t, func(tb *tbtest.Recorder) { HistogramPoints[float64](tb, rec, "test.size") })

	assert.True(t, failed.Failed())
	assert.Contains(t, failed.Messages(), "metric test.size is a metricdata.Histogram[int64], no histogram of float64")
}

func TestNew_AppliesTelemetryViews(t *testing.T) {
	t.Parallel()
	rec := New(t)
	histogram, err := rec.MeterProvider().Meter("pkgsilo/test").Float64Histogram("http.server.request.duration")
	require.NoError(t, err)
	histogram.Record(t.Context(), 1, metric.WithAttributes(semconv.ServerAddress("example.com"), semconv.HTTPRoute("/")))

	points := HistogramPoints[float64](t, rec, "http.server.request.duration")

	require.Len(t, points, 1)
	assert.Equal(t, []attribute.KeyValue{semconv.HTTPRoute("/")}, points[0].Attributes.ToSlice())
}
