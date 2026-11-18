# Telemetry

`core/telemetry` sets up the OpenTelemetry `MeterProvider`, which serves its metrics to Prometheus on the `METRICS_ADDR` listener, and the `TracerProvider`, which exports its spans over OTLP/HTTP when `OTEL_EXPORTER_OTLP_ENDPOINT` or `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is set.

## Providers are parameters

A component that records metrics takes a `metric.MeterProvider` as a parameter, like its logger, and one that creates spans takes a `trace.TracerProvider`; each creates its instruments and tracers from them.
The global providers that `cmd/main.go` sets are for third-party code only.

The one exception is `database.InTx`, which takes no provider and starts its transaction span from the span in its context (see its doc comment).
The queries in a transaction use the context that `InTx` passes to `fn`, which holds that span.

## Spans come from `core`

Spans come from `core` only: the server span of a request, authentication, authorization, transactions and queries.
Features create none.

A span of `core` whose operation fails sets its status with `span.SetStatus(codes.Error, …)` itself, since shared `core` packages log nothing.

## Names and attributes

- Name each instrument, span and attribute after the OpenTelemetry semantic conventions wherever one exists, and `pkgsilo.<area>.*` otherwise, such as the metric `pkgsilo.authn.attempts` or a span `pkgsilo.<area>.<operation>`.
- A span reuses the attribute keys of its component's metrics, as `pkgsilo.authn.authenticate` does those of `pkgsilo.authn.attempts`.
- Metric attributes take bounded values only: route patterns, methods, status codes, enum-like reasons.
  IDs, usernames, raw paths, SQL and error messages are unbounded.
  That holds for the attributes a library adds too: `telemetry.Views` drops those it can't bound, such as otelhttp's `server.address`, which comes from the client's Host header.
- Span attributes may hold IDs, but never credentials, hashes, peppers, query parameters, or request or response bodies.

## Errors reach spans through the logger

A log record at error level whose context holds a recording span adds an `exception` event to it and sets its status to `Error`, and every record with such a span names it with `trace_id` and `span_id`.
So log an error at error level with the request's context, `r.Context()`, under the key `error`.
The logger is the only path: a `span.RecordError` by hand would record the error twice.

## Tests

A test passes the providers of `telemetrytest.New(t)` and asserts the exact values they collect: metrics through an SDK `ManualReader` behind the views of `telemetry.New`, spans through `Spans()`, which holds every span once it ends.
An end-to-end test scrapes the app through `apptest.NewWithMetrics` and takes its spans from `apptest.NewWithTracing`.
