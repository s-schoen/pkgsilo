package telemetrytest

import (
	"context"
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

func TestSumPoints(t *testing.T) {
	t.Parallel()
	rec := New(t)
	meter := rec.MeterProvider().Meter("pkgsilo/test")
	gauge, err := meter.Int64ObservableUpDownCounter("test.count")
	require.NoError(t, err)
	_, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		o.ObserveInt64(gauge, 3, metric.WithAttributes(attribute.String("a", "1")))
		o.ObserveInt64(gauge, 5, metric.WithAttributes(attribute.String("a", "2")))

		return nil
	}, gauge)
	require.NoError(t, err)

	points := SumPoints[int64](t, rec, "test.count")

	require.Len(t, points, 2)
	byA := map[string]int64{}
	for _, p := range points {
		a, _ := p.Attributes.Value("a")
		byA[a.AsString()] = p.Value
	}
	assert.Equal(t, map[string]int64{"1": 3, "2": 5}, byA)
}

func TestSumPoints_NothingRecorded(t *testing.T) {
	t.Parallel()
	rec := New(t)
	_, err := rec.MeterProvider().Meter("pkgsilo/test").Int64Counter("test.requests")
	require.NoError(t, err)

	assert.Nil(t, SumPoints[int64](t, rec, "test.requests"))
}

func TestSumPoints_WrongType(t *testing.T) {
	t.Parallel()
	rec := New(t)
	counter, err := rec.MeterProvider().Meter("pkgsilo/test").Float64Counter("test.seconds")
	require.NoError(t, err)
	counter.Add(t.Context(), 1)

	failed := tbtest.Run(t, func(tb *tbtest.Recorder) { SumPoints[int64](tb, rec, "test.seconds") })

	assert.True(t, failed.Failed())
	assert.Contains(t, failed.Messages(), "metric test.seconds is a metricdata.Sum[float64], no sum of int64")
}

func TestSpans(t *testing.T) {
	t.Parallel()
	rec := New(t)
	tracer := rec.TracerProvider().Tracer("pkgsilo/test")

	ctx, parent := tracer.Start(t.Context(), "test.parent")
	_, child := tracer.Start(ctx, "test.child")
	child.End()
	_, unended := tracer.Start(ctx, "test.unended")
	parent.End()

	spans := rec.Spans()

	require.Len(t, spans, 2)
	assert.Equal(t, "test.child", spans[0].Name())
	assert.Equal(t, "test.parent", spans[1].Name())
	assert.Equal(t, spans[1].SpanContext().SpanID(), spans[0].Parent().SpanID())
	assert.True(t, spans[0].SpanContext().IsSampled())
	unended.End()
}

func TestNew_SamplesWhateverTheEnvironmentSays(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER", "always_off")
	rec := New(t)

	_, span := rec.TracerProvider().Tracer("pkgsilo/test").Start(t.Context(), "test.operation")
	span.End()

	assert.Len(t, rec.Spans(), 1)
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
