package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/semconv/v1.43.0/dbconv"
)

const (
	// meterName is the instrumentation scope of the pool metrics.
	meterName = "pkgsilo/core/database"
	// waitsName and waitDurationName count the requests for a connection that had to wait for one, and the
	// time they waited. semconv has only the histogram db.client.connection.wait_time, which database/sql
	// can't feed: it reports only these two totals. The names follow db.client.connection.timeouts.
	waitsName        = "db.client.connection.waits"
	waitDurationName = "db.client.connection.wait_duration"
)

// registerPoolMetrics registers the semconv connection pool metrics of db, read from db.Stats() whenever
// the provider collects, under the pool name pool:
//
//   - db.client.connection.count, by db.client.connection.state idle or used
//   - db.client.connection.max
//   - db.client.connection.waits and db.client.connection.wait_duration, see waitsName
//
// The returned function unregisters them, after which the pool reports nothing.
func registerPoolMetrics(meter metric.Meter, db *sql.DB, pool string) (unregister func() error, err error) {
	count, err := dbconv.NewClientConnectionCountObservable(meter)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", count.Name(), err)
	}
	maxConns, err := dbconv.NewClientConnectionMaxObservable(meter)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", maxConns.Name(), err)
	}
	waits, err := meter.Int64ObservableCounter(waitsName, metric.WithUnit("{wait}"),
		metric.WithDescription("The number of requests for a connection that had to wait for one."))
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", waitsName, err)
	}
	waitDuration, err := meter.Float64ObservableCounter(waitDurationName, metric.WithUnit("s"),
		metric.WithDescription("The time that requests for a connection waited for one, in total."))
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", waitDurationName, err)
	}

	poolAttr := count.AttrClientConnectionPoolName(pool)
	ofPool := metric.WithAttributeSet(attribute.NewSet(poolAttr))
	idle := metric.WithAttributeSet(attribute.NewSet(
		poolAttr, count.AttrClientConnectionState(dbconv.ClientConnectionStateIdle)))
	used := metric.WithAttributeSet(attribute.NewSet(
		poolAttr, count.AttrClientConnectionState(dbconv.ClientConnectionStateUsed)))

	registration, err := meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		stats := db.Stats()
		o.ObserveInt64(count.Inst(), int64(stats.Idle), idle)
		o.ObserveInt64(count.Inst(), int64(stats.InUse), used)
		o.ObserveInt64(maxConns.Inst(), int64(stats.MaxOpenConnections), ofPool)
		o.ObserveInt64(waits, stats.WaitCount, ofPool)
		o.ObserveFloat64(waitDuration, stats.WaitDuration.Seconds(), ofPool)

		return nil
	}, count.Inst(), maxConns.Inst(), waits, waitDuration)
	if err != nil {
		return nil, fmt.Errorf("register pool metrics: %w", err)
	}

	return func() error {
		if err := registration.Unregister(); err != nil {
			return fmt.Errorf("unregister pool metrics: %w", err)
		}

		return nil
	}, nil
}

// poolName returns the db.client.connection.pool.name of the pool that connects with connCfg. pgx
// names no pools, so it's server.address:server.port/db.namespace, as semconv suggests.
func poolName(connCfg *pgx.ConnConfig) string {
	return fmt.Sprintf("%s:%d/%s", connCfg.Host, connCfg.Port, connCfg.Database)
}
