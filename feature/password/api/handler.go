// Package api serves the password endpoints of the HTTP API.
package api

import (
	"log/slog"
	"net/http"
	"uuid"

	"github.com/go-chi/chi/v5"

	"pkgsilo/core/authn"
	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/feature/password/service"
	sessionapi "pkgsilo/feature/session/api"
)

// Handler serves the password endpoints.
type Handler struct {
	service service.PasswordService
	logger  *slog.Logger
}

// NewHandler returns a Handler that manages passwords through svc and logs to logger.
func NewHandler(svc service.PasswordService, logger *slog.Logger) *Handler {
	return &Handler{service: svc, logger: logger}
}

// Routes returns the password routes, to be mounted by the caller on a pattern that holds the ID of the
// password's identity as {identityID}.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.get)
	r.Put("/", h.set)
	r.Delete("/", h.delete)

	return r
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	identity, ok := identityID(w, r)
	if !ok {
		return
	}

	password, err := h.service.Get(r.Context(), authn.Actor(r), identity)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toPasswordResponse(password))
}

func (h *Handler) set(w http.ResponseWriter, r *http.Request) {
	identity, ok := identityID(w, r)
	if !ok {
		return
	}

	var body SetPasswordRequest
	if !request.DecodeJSON(w, r, &body) {
		return
	}

	password, err := h.service.Set(r.Context(), authn.Actor(r), identity, body.toInput())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	// The identity's sessions have ended, the request's own included if it has one.
	sessionapi.ExpireCookieOf(w, r, identity)
	envelope.WriteResource(w, toPasswordResponse(password))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	identity, ok := identityID(w, r)
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), authn.Actor(r), identity); err != nil {
		h.writeError(w, r, err)
		return
	}

	sessionapi.ExpireCookieOf(w, r, identity)
	w.WriteHeader(http.StatusNoContent)
}

// identityID returns the ID of the password's identity from the request path, a UUID or request.Me for the actor,
// answering any other value like an unknown identity.
func identityID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return request.IdentityParam(w, r, "identityID", identityapi.IdentityNotFoundProblem)
}
