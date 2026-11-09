package authz

import "go.opentelemetry.io/otel/attribute"

const (
	// tracerName is the instrumentation scope of the authorization span, named like the meter.
	tracerName = meterName
	// loadSpanName is the span of Engine.For, which loads the policies and the actor's groups. semconv has
	// no authorization spans, so it's pkgsilo's own.
	loadSpanName = "pkgsilo.authz.load"
	// reloadedKey tells whether For parsed the policies again, because their revision changed.
	reloadedKey = attribute.Key("pkgsilo.authz.policies.reloaded")
)
