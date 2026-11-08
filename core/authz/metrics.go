package authz

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	// meterName is the instrumentation scope of the authorization metrics.
	meterName = "pkgsilo/core/authz"
	// decisionsName counts the decisions of Engine, by result. semconv has no authorization metrics, so
	// it's pkgsilo's own.
	decisionsName = "pkgsilo.authz.decisions"

	decisionKey = attribute.Key("pkgsilo.authz.decision")
)

// decisions records pkgsilo.authz.decisions, with the single attribute pkgsilo.authz.decision: allow,
// deny, or error for a forbid policy that fails to evaluate.
//
// It names neither the action nor the resource on purpose: the ratio of denials is what an operator
// watches, and most denials are ordinary, such as the fields and rows that a read hides.
type decisions struct {
	counter metric.Int64Counter
	allow   metric.AddOption
	deny    metric.AddOption
	failed  metric.AddOption
}

// newDecisions creates the instrument with provider. It panics if the instrument can't be created, which
// only a bug in its definition can cause.
func newDecisions(provider metric.MeterProvider) decisions {
	counter, err := provider.Meter(meterName).Int64Counter(decisionsName, metric.WithUnit("{decision}"),
		metric.WithDescription("The number of authorization decisions, by result."))
	if err != nil {
		panic(fmt.Sprintf("authz: create %s: %v", decisionsName, err))
	}

	return decisions{
		counter: counter,
		allow:   metric.WithAttributeSet(attribute.NewSet(decisionKey.String("allow"))),
		deny:    metric.WithAttributeSet(attribute.NewSet(decisionKey.String("deny"))),
		failed:  metric.WithAttributeSet(attribute.NewSet(decisionKey.String("error"))),
	}
}

// record counts a decision: allowed, unless err is set.
//
// Decider.Can takes no context, and a Decider doesn't keep the one of For, so it records with the
// background context. That only matters for exemplars, which need the request's span.
func (d decisions) record(allowed bool, err error) {
	result := d.deny
	switch {
	case err != nil:
		result = d.failed
	case allowed:
		result = d.allow
	}
	d.counter.Add(context.Background(), 1, result)
}
