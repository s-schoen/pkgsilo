package authn

const (
	// tracerName is the instrumentation scope of the authentication span, named like the meter.
	tracerName = meterName
	// authenticateSpanName is the span of Require around the authenticators. semconv has no authentication
	// spans, so it's pkgsilo's own. Its attributes are those of pkgsilo.authn.attempts (see attempts).
	authenticateSpanName = "pkgsilo.authn.authenticate"
)
