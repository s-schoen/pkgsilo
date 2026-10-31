// Package router builds the HTTP router with the middleware shared by all features.
package router

import (
	"log/slog"
	"net/http"
	"net/netip"
	"strings"

	"github.com/go-chi/chi/v5"

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
}

// New returns a chi router with the middleware shared by all features.
//
// Unmatched routes and methods are answered with Problem Details. Subrouters inherit these
// handlers only when mounted after New returns.
func New(opts Options, logger *slog.Logger) *chi.Mux {
	r := chi.NewRouter()
	// Order matters: RequestLogger reads the request ID and client IP from the context it
	// receives, so both must be set before it. Recoverer must sit inside RequestLogger so
	// that recovered panics are logged with their 500 status.
	r.Use(middleware.RequestID)
	r.Use(middleware.ClientIP(opts.TrustedProxies))
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.Recoverer(logger, opts.Dev))
	r.Use(middleware.SecurityHeaders)

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		envelope.WriteProblem(w, envelope.NotFoundProblem.New(envelope.Occurrence{}))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		for _, m := range allowedMethods(req) {
			w.Header().Add("Allow", m)
		}
		envelope.WriteProblem(w, envelope.MethodNotAllowedProblem.New(envelope.Occurrence{}))
	})

	return r
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
