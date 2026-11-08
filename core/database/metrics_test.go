package database_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"pkgsilo/test/databasetest"
	"pkgsilo/test/telemetrytest"
)

// queryAttrs are the attributes of the operation metrics of a query: otelpgx's defaults, nothing else.
var queryAttrs = attribute.NewSet(
	attribute.String("db.system.name", "postgresql"),
	attribute.String("pgx.operation.type", "query"),
)

// byState returns the values of the db.client.connection.count points by db.client.connection.state,
// and fails the test if a point lacks the pool name.
func byState(t *testing.T, points []metricdata.DataPoint[int64]) map[string]int64 {
	t.Helper()

	states := map[string]int64{}
	for _, p := range points {
		state, _ := p.Attributes.Value("db.client.connection.state")
		states[state.AsString()] = p.Value
		assert.True(t, p.Attributes.HasValue("db.client.connection.pool.name"), "point without a pool name")
	}

	return states
}

// value returns the value of the only point, failing the test if there's not exactly one.
func value[N int64 | float64](t *testing.T, points []metricdata.DataPoint[N]) N {
	t.Helper()
	require.Len(t, points, 1)

	return points[0].Value
}

func TestOpen_PoolMetrics(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := databasetest.NewWithMeterProvider(t, rec.MeterProvider())

	conn, err := db.Connx(t.Context())
	require.NoError(t, err)

	assert.Equal(t, map[string]int64{"idle": 0, "used": 1},
		byState(t, telemetrytest.SumPoints[int64](t, rec, "db.client.connection.count")))
	assert.Equal(t, int64(db.Stats().MaxOpenConnections),
		value(t, telemetrytest.SumPoints[int64](t, rec, "db.client.connection.max")))
	assert.Zero(t, value(t, telemetrytest.SumPoints[int64](t, rec, "db.client.connection.waits")))

	require.NoError(t, conn.Close())

	assert.Equal(t, map[string]int64{"idle": 1, "used": 0},
		byState(t, telemetrytest.SumPoints[int64](t, rec, "db.client.connection.count")))
}

// TestOpen_PoolMetricsWaits checks that a query that waits for a connection, because the pool's
// connections are all held, is counted with the time it waited.
func TestOpen_PoolMetricsWaits(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := databasetest.NewWithMeterProvider(t, rec.MeterProvider())
	held := make([]*sql.Conn, db.Stats().MaxOpenConnections)
	for i := range held {
		var err error
		held[i], err = db.Conn(t.Context())
		require.NoError(t, err)
	}

	queried := make(chan error)
	go func() {
		var n int
		queried <- db.GetContext(t.Context(), &n, "SELECT 1")
	}()
	require.Eventually(t, func() bool { return db.Stats().WaitCount == 1 }, 5*time.Second, time.Millisecond)
	time.Sleep(10 * time.Millisecond) // So that the wait takes measurably long.
	require.NoError(t, held[0].Close())
	require.NoError(t, <-queried)

	assert.Equal(t, int64(1), value(t, telemetrytest.SumPoints[int64](t, rec, "db.client.connection.waits")))
	assert.GreaterOrEqual(t,
		value(t, telemetrytest.SumPoints[float64](t, rec, "db.client.connection.wait_duration")), 0.01)
	for _, conn := range held[1:] {
		require.NoError(t, conn.Close())
	}
}

// TestOpen_PoolMetricsStopOnClose checks that a closed database reports no pool metrics, so that its
// callback is gone.
func TestOpen_PoolMetricsStopOnClose(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := databasetest.NewWithMeterProvider(t, rec.MeterProvider())
	require.NotNil(t, telemetrytest.SumPoints[int64](t, rec, "db.client.connection.count"))

	require.NoError(t, db.Close())

	for _, name := range []string{
		"db.client.connection.count", "db.client.connection.max",
		"db.client.connection.waits", "db.client.connection.wait_duration",
	} {
		_, ok := rec.Metric(t, name)
		assert.False(t, ok, name)
	}
}

func TestOpen_OperationMetrics(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := databasetest.NewWithMeterProvider(t, rec.MeterProvider())

	var n int
	require.NoError(t, db.GetContext(t.Context(), &n, "SELECT 1"))

	var queries uint64
	for _, p := range telemetrytest.HistogramPoints[float64](t, rec, "db.client.operation.duration") {
		if p.Attributes.Equals(&queryAttrs) {
			queries = p.Count
		}
	}
	assert.Equal(t, uint64(1), queries)
	assert.Nil(t, telemetrytest.SumPoints[int64](t, rec, "db.client.operation.errors"))
}

func TestOpen_OperationMetricsErrors(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := databasetest.NewWithMeterProvider(t, rec.MeterProvider())

	var n int
	require.Error(t, db.GetContext(t.Context(), &n, "SELECT 1/0"))

	points := telemetrytest.SumPoints[int64](t, rec, "db.client.operation.errors")
	require.Len(t, points, 1)
	assert.Equal(t, queryAttrs, points[0].Attributes)
	assert.Equal(t, int64(1), points[0].Value)
}

// TestOpen_OperationMetricsNoRows checks that a query that finds no row is no error.
func TestOpen_OperationMetricsNoRows(t *testing.T) {
	t.Parallel()
	rec := telemetrytest.New(t)
	db := databasetest.NewWithMeterProvider(t, rec.MeterProvider())

	var n int
	require.ErrorIs(t, db.GetContext(t.Context(), &n, "SELECT 1 WHERE false"), sql.ErrNoRows)

	assert.Nil(t, telemetrytest.SumPoints[int64](t, rec, "db.client.operation.errors"))
}
