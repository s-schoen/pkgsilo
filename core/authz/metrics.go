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
	// decisionsName counts the decisions of Engine, by purpose and result. semconv has no authorization
	// metrics, so it's pkgsilo's own.
	decisionsName = "pkgsilo.authz.decisions"

	decisionKey = attribute.Key("pkgsilo.authz.decision")
	purposeKey  = attribute.Key("pkgsilo.authz.purpose")
)

// purpose is what a decision is for: the value of the attribute pkgsilo.authz.purpose.
type purpose int

const (
	// purposeEnforce is a decision that a request is allowed or denied by, from Decider.Can.
	purposeEnforce purpose = iota
	// purposeHint is a decision that only tells a client what it may do, from Decider.Hint.
	purposeHint

	purposes
)

// result is the result of a decision: the value of the attribute pkgsilo.authz.decision.
type result int

const (
	resultAllow result = iota
	resultDeny
	resultError

	results
)

// decisions records pkgsilo.authz.decisions, with the attributes pkgsilo.authz.purpose: enforce for a check
// that a request is allowed or denied by, hint for one that only tells a client what it may do, such as the
// allowed actions of an object; and pkgsilo.authz.decision: allow, deny, or error for a forbid policy that
// fails to evaluate.
//
// It names neither the action nor the resource on purpose: the ratio of denials is what an operator
// watches, and most denials are ordinary, such as the fields and rows that a read hides. The purpose keeps
// the hints, which deny as a matter of course, out of the ratio of the enforced decisions.
type decisions struct {
	counter metric.Int64Counter
	// options holds the attributes of each purpose and result.
	options [purposes][results]metric.AddOption
}

// newDecisions creates the instrument with provider. It panics if the instrument can't be created, which
// only a bug in its definition can cause.
func newDecisions(provider metric.MeterProvider) decisions {
	counter, err := provider.Meter(meterName).Int64Counter(decisionsName, metric.WithUnit("{decision}"),
		metric.WithDescription("The number of authorization decisions, by purpose and result."))
	if err != nil {
		panic(fmt.Sprintf("authz: create %s: %v", decisionsName, err))
	}

	d := decisions{counter: counter}
	purposeNames := [purposes]string{purposeEnforce: "enforce", purposeHint: "hint"}
	resultNames := [results]string{resultAllow: "allow", resultDeny: "deny", resultError: "error"}
	for p, purposeName := range purposeNames {
		for r, resultName := range resultNames {
			d.options[p][r] = metric.WithAttributeSet(attribute.NewSet(
				purposeKey.String(purposeName), decisionKey.String(resultName)))
		}
	}

	return d
}

// record counts a decision for the purpose: allowed, unless err is set.
//
// Decider.Can takes no context, and a Decider doesn't keep the one of For, so it records with the
// background context. That only matters for exemplars, which need the request's span.
func (d decisions) record(p purpose, allowed bool, err error) {
	r := resultDeny
	switch {
	case err != nil:
		r = resultError
	case allowed:
		r = resultAllow
	}
	d.counter.Add(context.Background(), 1, d.options[p][r])
}
