// Package router builds the HTTP router with the middleware shared by all features.
package router

import (
	"log/slog"
	"net/http"
	"net/netip"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/envelope"
	"pkgsilo/core/middleware"
)

// Options configures the router.
type Options struct {
	// TrustedProxies are the reverse proxies whose X-Forwarded-For header determines the
	// client IP. Empty: the header is ignored.
	TrustedProxies []netip.Prefix
	// Dev prints the stack traces of recovered panics in a readable form instead of logging them.
	Dev bool
	// MeterProvider records the HTTP server metrics of every request. Nil: nothing is recorded.
	MeterProvider metric.MeterProvider
	// UnmeasuredPaths are the paths whose requests aren't measured, such as the health check, which
	// would only measure the health checker.
	UnmeasuredPaths []string
}

// New returns a chi router with the middleware shared by all features.
//
// Unmatched routes and methods are answered with Problem Details. Subrouters inherit these
// handlers only when mounted after New returns.
func New(opts Options, logger *slog.Logger) *chi.Mux {
	r := chi.NewRouter()
	// Order matters: the metrics middleware comes first, so that it measures what the middleware
	// below writes too, such as the 500 of a recovered panic. RequestLogger reads the request ID
	// and client IP from the context it receives, so both must be set before it. Recoverer must
	// sit inside RequestLogger so that recovered panics are logged with their 500 status.
	r.Use(measure(opts))
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

// measure returns the middleware that records the HTTP server metrics of semconv with the provider
// of opts, http.route being the chi route pattern of the request.
//
// It must run as a middleware of the root router: chi returns its route context to a pool once
// the router's ServeHTTP returns, and the route is known only once the request is answered.
// otelhttp's own route, from Request.Pattern, can't be used: chi sets it only on the copy of the
// request that it passes to the route's handler.
func measure(opts Options) func(http.Handler) http.Handler {
	provider := opts.MeterProvider
	if provider == nil {
		provider = noop.NewMeterProvider()
	}
	unmeasured := make(map[string]bool, len(opts.UnmeasuredPaths))
	for _, path := range opts.UnmeasuredPaths {
		unmeasured[path] = true
	}

	otel := otelhttp.NewMiddleware("http.server",
		otelhttp.WithMeterProvider(provider),
		// Spans come with tracing, which isn't set up: until then, none are created.
		otelhttp.WithTracerProvider(tracenoop.NewTracerProvider()),
		otelhttp.WithFilter(func(r *http.Request) bool { return !unmeasured[r.URL.Path] }),
	)

	return func(next http.Handler) http.Handler {
		return otel(labelRoute(next))
	}
}

// labelRoute adds the request's route pattern as http.route to the labeler of otelhttp, which reads it
// once next returns.
func labelRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if route := chi.RouteContext(r.Context()).RoutePattern(); route != "" {
			labeler, _ := otelhttp.LabelerFromContext(r.Context())
			labeler.Add(semconv.HTTPRoute(route))
		}
	})
}

// forgetRoute drops the route patterns that chi recorded for the request, so that its metrics have no
// http.route. A request that matches no route still records the patterns of the subrouters on its way,
// such as /api/v1/identities/* for /api/v1/identities/1/missing, which name no route.
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

	path := r.URL.RawPath
	if path == "" {
		path = r.URL.Path
	}

	return routedMethods(rctx.Routes, path)
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
