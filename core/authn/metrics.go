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

// reasons are the reasons that rejectionReason returns.
var reasons = []string{reasonMalformed, reasonUnknown, reasonExpired, reasonInvalid}

// attempts records pkgsilo.authn.attempts, with these attributes:
//
//   - pkgsilo.authn.outcome: success, missing (no method recognized a credential), rejected or error
//   - pkgsilo.authn.method: the method that decided, left out on missing
//   - pkgsilo.authn.reason: on rejected only, the reason of the rejection (see ErrInvalidCredentials)
//
// Its attribute sets are built once, for the methods of Require, so that no request builds and sorts one.
type attempts struct {
	counter      metric.Int64Counter
	missingAttrs metric.AddOption
	// methods holds the attribute sets of each method, at the method's index in Require.
	methods []methodAttempts
}

// methodAttempts holds the attribute sets of the attempts that one method decided.
type methodAttempts struct {
	success metric.AddOption
	failure metric.AddOption
	// rejection holds the attribute set of each of the reasons.
	rejection map[string]metric.AddOption
}

// newAttempts creates the instrument with provider, for the given methods. It panics if the instrument
// can't be created, which only a bug in its definition can cause.
func newAttempts(provider metric.MeterProvider, methods []string) attempts {
	counter, err := provider.Meter(meterName).Int64Counter(attemptsName, metric.WithUnit("{attempt}"),
		metric.WithDescription("The number of requests whose authentication was attempted, by outcome."))
	if err != nil {
		panic(fmt.Sprintf("authn: create %s: %v", attemptsName, err))
	}

	a := attempts{
		counter:      counter,
		missingAttrs: attrs(outcomeKey.String(outcomeMissing)),
		methods:      make([]methodAttempts, len(methods)),
	}
	for i, method := range methods {
		methodAttr := methodKey.String(method)
		a.methods[i] = methodAttempts{
			success:   attrs(outcomeKey.String(outcomeSuccess), methodAttr),
			failure:   attrs(outcomeKey.String(outcomeError), methodAttr),
			rejection: make(map[string]metric.AddOption, len(reasons)),
		}
		for _, reason := range reasons {
			a.methods[i].rejection[reason] = attrs(
				outcomeKey.String(outcomeRejected), methodAttr, reasonKey.String(reason))
		}
	}

	return a
}

// attrs returns the option that records with the attribute set of kvs.
func attrs(kvs ...attribute.KeyValue) metric.AddOption {
	return metric.WithAttributeSet(attribute.NewSet(kvs...))
}

// succeeded counts an attempt that the method at index method in Require accepted.
func (a attempts) succeeded(ctx context.Context, method int) {
	a.counter.Add(ctx, 1, a.methods[method].success)
}

// missing counts an attempt whose credential no method recognized.
func (a attempts) missing(ctx context.Context) {
	a.counter.Add(ctx, 1, a.missingAttrs)
}

// rejected counts an attempt that the method at index method in Require rejected for reason, one of
// reasons.
func (a attempts) rejected(ctx context.Context, method int, reason string) {
	a.counter.Add(ctx, 1, a.methods[method].rejection[reason])
}

// failed counts an attempt that the method at index method in Require failed to decide.
func (a attempts) failed(ctx context.Context, method int) {
	a.counter.Add(ctx, 1, a.methods[method].failure)
}
