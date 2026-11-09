// Package router builds the HTTP router with the middleware shared by all features.
package router

import (
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"pkgsilo/core/envelope"
	"pkgsilo/core/middleware"
)

// Options configures the router.
type Options struct {
	// TrustedProxies are the reverse proxies whose X-Forwarded-For header determines the
	// client IP, and whose traceparent header a request's trace continues. Empty: both
	// headers are ignored, and every request starts a new trace that links to the caller's.
	TrustedProxies []netip.Prefix
	// Dev prints the stack traces of recovered panics in a readable form instead of logging them.
	Dev bool
	// UnmeasuredPaths are the paths whose requests aren't measured and get no span, such as the
	// health check, which would only measure the health checker. A path matches with a trailing
	// slash too, since a subrouter mounted at it serves both.
	UnmeasuredPaths []string
}

// New returns a chi router with the middleware shared by all features, which records the HTTP server
// metrics of every request with meterProvider and its server span with tracerProvider. It panics if
// either provider is nil.
//
// Unmatched routes and methods are answered with Problem Details. Subrouters inherit these
// handlers only when mounted after New returns.
func New(
	opts Options, logger *slog.Logger, meterProvider metric.MeterProvider, tracerProvider trace.TracerProvider,
) *chi.Mux {
	if meterProvider == nil {
		panic("router: New needs a meter provider")
	}
	if tracerProvider == nil {
		panic("router: New needs a tracer provider")
	}

	r := chi.NewRouter()
	// Order matters: the telemetry middleware comes first, so that it measures what the middleware
	// below writes too, such as the 500 of a recovered panic, and so that RequestID and ClientIP
	// find its span. RequestLogger reads the request ID and client IP from the context it
	// receives, so both must be set before it. Recoverer must sit inside RequestLogger so that
	// recovered panics are logged with their 500 status.
	r.Use(measure(meterProvider, tracerProvider, opts))
	r.Use(middleware.RequestID)
	r.Use(middleware.ClientIP(opts.TrustedProxies))
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.Recoverer(logger, opts.Dev))
	r.Use(middleware.SecurityHeaders)

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		forgetRoute(req)
		envelope.WriteProblem(w, envelope.NotFoundProblem.New(envelope.Occurrence{}))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		for _, m := range allowedMethods(req) {
			w.Header().Add("Allow", m)
		}
		forgetRoute(req)
		envelope.WriteProblem(w, envelope.MethodNotAllowedProblem.New(envelope.Occurrence{}))
	})

	return r
}

// measure returns the middleware that records the HTTP server metrics of semconv with meterProvider and the
// server span with tracerProvider, for every request but those to opts.UnmeasuredPaths. http.route is the
// chi route pattern of the request, and the span is named after it.
//
// The span continues the trace of an incoming traceparent only if the request's direct peer is one of
// opts.TrustedProxies. Any other request starts a new trace that links to the caller's span, so that no
// client can choose its trace ID or force it to be sampled. With tracing off, a request's context holds no
// span context at all, not even a trusted proxy's, so that no trace reaches the exemplars of its metrics
// (see serverTracer.Start). Baggage isn't propagated.
//
// A 5xx response sets the span's status to Error, without erasing the description of an error that the
// logger recorded on it (see serverSpan.SetStatus).
//
// It must run as a middleware of the root router: chi returns its route context to a pool once
// the router's ServeHTTP returns, and the route is known only once the request is answered.
// otelhttp's own route, from Request.Pattern, can't be used: chi sets it only on the copy of the
// request that it passes to the route's handler.
func measure(
	meterProvider metric.MeterProvider, tracerProvider trace.TracerProvider, opts Options,
) func(http.Handler) http.Handler {
	unmeasured := make(map[string]bool, len(opts.UnmeasuredPaths))
	for _, path := range opts.UnmeasuredPaths {
		unmeasured[strings.TrimSuffix(path, "/")] = true
	}
	trusted := middleware.TrustedProxies(opts.TrustedProxies)

	otel := otelhttp.NewMiddleware("http.server",
		otelhttp.WithMeterProvider(meterProvider),
		otelhttp.WithTracerProvider(serverTracerProvider{tracerProvider}),
		// Never the global propagator, which cmd/main.go leaves unset.
		otelhttp.WithPropagators(propagation.TraceContext{}),
		otelhttp.WithPublicEndpointFn(func(r *http.Request) bool { return !trusted.ContainsPeer(r) }),
		// Named before chi routes the request: labelRoute adds the route once it's known.
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return spanMethod(r) }),
		// By the path that chi routes, so that a path it routes elsewhere is measured.
		otelhttp.WithFilter(func(r *http.Request) bool {
			return !unmeasured[strings.TrimSuffix(routingPath(r), "/")]
		}),
	)

	return func(next http.Handler) http.Handler {
		return otel(labelRoute(next))
	}
}

// labelRoute adds the request's route pattern as http.route to the labeler of otelhttp, which reads it
// once next returns, and to the server span, which it renames to "{method} {route}". A request without a
// route keeps its span's name.
func labelRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		route := routePattern(r)
		if route == "" {
			return
		}
		labeler, _ := otelhttp.LabelerFromContext(r.Context())
		labeler.Add(semconv.HTTPRoute(route))
		span := trace.SpanFromContext(r.Context())
		span.SetName(spanMethod(r) + " " + route)
		span.SetAttributes(semconv.HTTPRoute(route))
	})
}

// spanMethod returns the request's method as a server span's name begins with it: in upper case, or HTTP
// for a method that semconv doesn't know, which the metrics record as _OTHER.
func spanMethod(r *http.Request) string {
	method := strings.ToUpper(r.Method)
	if !slices.Contains(standardMethods, method) {
		return "HTTP"
	}

	return method
}

// routePattern returns the pattern of the route that the request matches, or "" if it matches none.
//
// A request that a middleware of a group answers, such as the 401 of authn.Require, never reaches the
// subrouter mounted in the group, so that chi recorded the mount's wildcard, such as
// /api/v1/identities/*, as the pattern. It's routed again from the root then, to the route that the
// subrouter would have served.
func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	pattern := rctx.RoutePattern()
	if !strings.HasSuffix(pattern, "/*") {
		return pattern
	}

	// A route that ends in a wildcard of its own is found again, under the same pattern.
	return rctx.Routes.Find(chi.NewRouteContext(), r.Method, routingPath(r))
}

// forgetRoute drops the route patterns that chi recorded for the request, so that its metrics have no
// http.route. A request that matches no route still records the patterns on its way, such as /sub for
// a request to /sub when the subrouter mounted there routes nothing at /, which name no route.
func forgetRoute(r *http.Request) {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		rctx.RoutePatterns = rctx.RoutePatterns[:0]
	}
}

var standardMethods = []string{
	http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
	http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace,
}

// allowedMethods returns the methods routed for the request's path.
//
// chi only sets the Allow header in its default 405 handler and doesn't expose the allowed
// methods to custom ones, so they are recovered by matching each method against the root router.
func allowedMethods(r *http.Request) []string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return nil
	}

	return routedMethods(rctx.Routes, routingPath(r))
}

// routingPath returns the path that the root router routes the request by, as chi does: the escaped
// path if it differs from the default escaping of the decoded path.
func routingPath(r *http.Request) string {
	if r.URL.RawPath != "" {
		return r.URL.RawPath
	}

	return r.URL.Path
}

func routedMethods(routes chi.Routes, path string) []string {
	var methods []string
	for _, m := range standardMethods {
		pattern := routes.Find(chi.NewRouteContext(), m, path)
		if pattern == "" {
			continue
		}
		// Mount registers catch-all stubs for the bare mount path, which Find doesn't follow
		// into the subrouter. Requests to them are routed to the subrouter's root.
		if sub := mountedAt(routes, pattern); sub != nil {
			return routedMethods(sub, "/")
		}
		methods = append(methods, m)
	}

	return methods
}

// mountedAt returns the subrouter mounted at pattern, or nil if there is none.
func mountedAt(routes chi.Routes, pattern string) chi.Routes {
	mount := strings.TrimSuffix(pattern, "/") + "/*"
	for _, rt := range routes.Routes() {
		if rt.SubRoutes == nil {
			continue
		}
		if rt.Pattern == mount {
			return rt.SubRoutes
		}
		// Find joins the patterns of nested mounts, so a mount inside a subrouter is looked up
		// there by the rest of the pattern.
		rest, ok := strings.CutPrefix(pattern, strings.TrimSuffix(rt.Pattern, "/*"))
		if !ok || !strings.HasPrefix(rest, "/") {
			continue
		}
		if sub := mountedAt(rt.SubRoutes, rest); sub != nil {
			return sub
		}
	}

	return nil
}
