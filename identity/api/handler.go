// Package api serves the identity endpoints of the HTTP API.
package api

import (
	"log/slog"
	"net/http"
	"path"
	"uuid"

	"github.com/go-chi/chi/v5"

	"pkgsilo/envelope"
	"pkgsilo/identity/service"
	"pkgsilo/request"
)

// Handler serves the identity endpoints.
type Handler struct {
	service service.IdentityService
	logger  *slog.Logger
}

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

	// One identity more than requested reveals whether a next page exists.
	identities, err := h.service.List(r.Context(), page.Offset, page.Limit+1)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	data := make([]IdentityResponse, len(identities))
	for i, identity := range identities {
		data[i] = toIdentityResponse(identity)
	}
	c, err := envelope.NewCollectionFromLookahead(data, envelope.LookaheadOptions{Offset: page.Offset, Limit: page.Limit})
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteJSON(w, http.StatusOK, c)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body CreateIdentityRequest
	if !request.DecodeJSON(w, r, &body) {
		return
	}

	identity, err := h.service.Create(r.Context(), service.SystemActor, body.toInput())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	w.Header().Set("Location", path.Join(r.URL.Path, identity.ID.String()))
	envelope.WriteJSON(w, http.StatusCreated, envelope.NewResource(toIdentityResponse(identity)))
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

	envelope.WriteJSON(w, http.StatusOK, envelope.NewResource(toIdentityResponse(identity)))
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

	identity, err := h.service.Replace(r.Context(), service.SystemActor, id, body.toInput())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteJSON(w, http.StatusOK, envelope.NewResource(toIdentityResponse(identity)))
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

// identityID returns the identity ID from the request path. An ID that isn't a UUID can't
// identify an identity, so it's answered with 404 like an unknown one.
func identityID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		envelope.WriteProblem(w, identityNotFoundProblem)
		return uuid.UUID{}, false
	}

	return id, true
}
