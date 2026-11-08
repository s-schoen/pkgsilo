package authn

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	// meterName is the instrumentation scope of the authentication metrics.
	meterName = "pkgsilo/core/authn"
	// attemptsName counts the requests that Require handled, by outcome. semconv has no authentication
	// metrics, so it's pkgsilo's own.
	attemptsName = "pkgsilo.authn.attempts"

	outcomeKey = attribute.Key("pkgsilo.authn.outcome")
	methodKey  = attribute.Key("pkgsilo.authn.method")
	reasonKey  = attribute.Key("pkgsilo.authn.reason")

	outcomeSuccess  = "success"
	outcomeMissing  = "missing"
	outcomeRejected = "rejected"
	outcomeError    = "error"
)

// attempts records pkgsilo.authn.attempts, with these attributes:
//
//   - pkgsilo.authn.outcome: success, missing (no method recognized a credential), rejected or error
//   - pkgsilo.authn.method: the method that decided, left out on missing
//   - pkgsilo.authn.reason: on rejected only, the reason of the rejection (see ErrInvalidCredentials)
type attempts struct {
	counter metric.Int64Counter
}

// newAttempts creates the instrument with provider. It panics if the instrument can't be created,
// which only a bug in its definition can cause.
func newAttempts(provider metric.MeterProvider) attempts {
	counter, err := provider.Meter(meterName).Int64Counter(attemptsName, metric.WithUnit("{attempt}"),
		metric.WithDescription("The number of requests whose authentication was attempted, by outcome."))
	if err != nil {
		panic(fmt.Sprintf("authn: create %s: %v", attemptsName, err))
	}

	return attempts{counter: counter}
}

func (a attempts) succeeded(ctx context.Context, method string) {
	a.add(ctx, outcomeKey.String(outcomeSuccess), methodKey.String(method))
}

func (a attempts) missing(ctx context.Context) {
	a.add(ctx, outcomeKey.String(outcomeMissing))
}

func (a attempts) rejected(ctx context.Context, method, reason string) {
	a.add(ctx, outcomeKey.String(outcomeRejected), methodKey.String(method), reasonKey.String(reason))
}

func (a attempts) failed(ctx context.Context, method string) {
	a.add(ctx, outcomeKey.String(outcomeError), methodKey.String(method))
}

func (a attempts) add(ctx context.Context, attrs ...attribute.KeyValue) {
	a.counter.Add(ctx, 1, metric.WithAttributes(attrs...))
}
