package core

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
)

// NewRouter returns a chi router with the middleware shared by all features.
func NewRouter(cfg Config, logger *slog.Logger) *chi.Mux {
	r := chi.NewRouter()
	// Order matters: RequestLogger reads the request ID and client IP from the context it
	// receives, so both must be set before it. Recoverer must sit inside RequestLogger so
	// that recovered panics are logged with their 500 status.
	r.Use(RequestID)
	r.Use(ClientIP(cfg.TrustedProxies))
	r.Use(RequestLogger(logger))
	r.Use(Recoverer(logger, cfg.IsDev()))
	r.Use(SecurityHeaders)

	return r
}
