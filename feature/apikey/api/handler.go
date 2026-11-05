// Package api serves the API key endpoints of the HTTP API and authenticates requests by their API
// key.
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
	"pkgsilo/feature/apikey/service"
	identityapi "pkgsilo/feature/identity/api"
)

// Handler serves the API key endpoints.
type Handler struct {
	service service.APIKeyService
	logger  *slog.Logger
}

// NewHandler returns a Handler that manages API keys through svc and logs to logger.
func NewHandler(svc service.APIKeyService, logger *slog.Logger) *Handler {
	return &Handler{service: svc, logger: logger}
}

// Routes returns the API key routes, to be mounted by the caller on a pattern that holds the ID of
// the API keys' identity as {identityID}.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Delete("/{id}", h.delete)

	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	identity, ok := identityID(w, r)
	if !ok {
		return
	}

	page, ok := request.ParsePagination(w, r)
	if !ok {
		return
	}

	keys, err := h.service.List(r.Context(), identity, page.Offset, page.Limit)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	data := make([]KeyResponse, len(keys.Items))
	for i, key := range keys.Items {
		data[i] = toKeyResponse(key)
	}

	envelope.WriteCollection(w, envelope.NewPagedCollection(data, page.Offset, page.Limit, keys.HasMore))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	identity, ok := identityID(w, r)
	if !ok {
		return
	}

	var body CreateKeyRequest
	if !request.DecodeJSON(w, r, &body) {
		return
	}

	key, err := h.service.Create(r.Context(), audit.SystemActor(), identity, body.toInput())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	// The only response that holds the key must not end up in a cache.
	w.Header().Set("Cache-Control", "no-store")
	envelope.WriteCreated(w, path.Join(r.URL.Path, key.ID.String()), toCreatedKeyResponse(key))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	identity, ok := identityID(w, r)
	if !ok {
		return
	}

	id, ok := apiKeyID(w, r)
	if !ok {
		return
	}

	key, err := h.service.Get(r.Context(), identity, id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toKeyResponse(key))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	identity, ok := identityID(w, r)
	if !ok {
		return
	}

	id, ok := apiKeyID(w, r)
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), identity, id); err != nil {
		h.writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// identityID returns the ID of the API keys' identity from the request path, answering one that
// isn't a UUID like an unknown one.
func identityID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return request.UUIDParam(w, r, "identityID", identityapi.IdentityNotFoundProblem)
}

// apiKeyID returns the API key ID from the request path, answering one that isn't a UUID like an
// unknown one.
func apiKeyID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return request.UUIDParam(w, r, "id", apiKeyNotFoundProblem)
}
