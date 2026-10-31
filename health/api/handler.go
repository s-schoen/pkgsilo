// Package api serves the health endpoint of the HTTP API.
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pkgsilo/core/envelope"
	"pkgsilo/health/service"
)

// Handler serves the health check endpoint.
type Handler struct {
	service service.HealthService
	logger  *slog.Logger
}

func NewHandler(svc service.HealthService, logger *slog.Logger) *Handler {
	return &Handler{service: svc, logger: logger}
}

// Routes returns the health routes, to be mounted by the caller.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.check)

	return r
}

func (h *Handler) check(w http.ResponseWriter, r *http.Request) {
	health := h.service.Check(r.Context())

	// Causes are logged, never exposed: they may contain internal addresses.
	for _, c := range health.Components {
		if c.Cause != nil {
			h.logger.ErrorContext(r.Context(), "health component unhealthy", "component", c.Name, "error", c.Cause)
		}
	}

	resp := toHealthResponse(health)
	envelope.WriteJSON(w, resp.httpStatus(), resp)
}
