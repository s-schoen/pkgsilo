// Package api serves the identity endpoints of the HTTP API.
package api

import (
	"log/slog"
	"net/http"
	"path"
	"uuid"

	"github.com/go-chi/chi/v5"

	"pkgsilo/core/audit"
	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	"pkgsilo/feature/identity/service"
)

// Handler serves the identity endpoints.
type Handler struct {
	service service.IdentityService
	logger  *slog.Logger
}

// NewHandler returns a Handler that manages identities through svc and logs to logger.
func NewHandler(svc service.IdentityService, logger *slog.Logger) *Handler {
	return &Handler{service: svc, logger: logger}
}

// Routes returns the identity routes, to be mounted by the caller.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.replace)
	r.Delete("/{id}", h.delete)

	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, ok := request.ParsePagination(w, r)
	if !ok {
		return
	}

	identities, err := h.service.List(r.Context(), page.Offset, page.Limit)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	data := make([]IdentityResponse, len(identities.Items))
	for i, identity := range identities.Items {
		data[i] = toIdentityResponse(identity)
	}

	envelope.WriteCollection(w, envelope.NewPagedCollection(data, page.Offset, page.Limit, identities.HasMore))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body CreateIdentityRequest
	if !request.DecodeJSON(w, r, &body) {
		return
	}

	identity, err := h.service.Create(r.Context(), audit.SystemActor(), body.toInput())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteCreated(w, path.Join(r.URL.Path, identity.ID.String()), toIdentityResponse(identity))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := identityID(w, r)
	if !ok {
		return
	}

	identity, err := h.service.Get(r.Context(), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toIdentityResponse(identity))
}

func (h *Handler) replace(w http.ResponseWriter, r *http.Request) {
	id, ok := identityID(w, r)
	if !ok {
		return
	}

	var body ReplaceIdentityRequest
	if !request.DecodeJSON(w, r, &body) {
		return
	}

	identity, err := h.service.Replace(r.Context(), audit.SystemActor(), id, body.toInput())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toIdentityResponse(identity))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := identityID(w, r)
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		h.writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// identityID returns the identity ID from the request path, answering one that isn't a UUID like
// an unknown one.
func identityID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return request.UUIDParam(w, r, "id", IdentityNotFoundProblem)
}
