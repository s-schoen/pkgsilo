package health

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"pkgsilo/core"
)

// Handler serves the health check endpoint.
type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

// Routes returns the health routes, to be mounted by the caller.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.check)

	return r
}

func (h *Handler) check(w http.ResponseWriter, _ *http.Request) {
	core.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
