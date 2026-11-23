// Package api serves the permissions endpoint of the HTTP API.
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pkgsilo/core/authn"
	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/feature/permission/service"
)

// ownerParam is the query parameter that asks for the actions on the resources that an identity owns.
const ownerParam = "owner"

// Handler serves the permissions endpoint.
type Handler struct {
	service service.PermissionService
	logger  *slog.Logger
}

// NewHandler returns a Handler that lists permissions through svc and logs to logger.
func NewHandler(svc service.PermissionService, logger *slog.Logger) *Handler {
	return &Handler{service: svc, logger: logger}
}

// Routes returns the permission routes, to be mounted by the caller on a pattern that holds the ID of the
// identity as {identityID}.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)

	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	identityID, ok := request.IdentityParam(w, r, "identityID", identityapi.IdentityNotFoundProblem)
	if !ok {
		return
	}

	owner, ok := request.IdentityQueryParam(w, r, ownerParam)
	if !ok {
		return
	}

	permissions, err := h.service.List(r.Context(), authn.Actor(r), identityID, owner)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toPermissionsResponse(permissions))
}
