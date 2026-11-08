// Package api serves the policy endpoints of the HTTP API.
package api

import (
	"log/slog"
	"net/http"
	"path"
	"uuid"

	"github.com/go-chi/chi/v5"

	"pkgsilo/core/authn"
	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	"pkgsilo/feature/policy/service"
)

// Handler serves the policy endpoints.
type Handler struct {
	service service.PolicyService
	logger  *slog.Logger
}

// NewHandler returns a Handler that manages policies through svc and logs to logger.
func NewHandler(svc service.PolicyService, logger *slog.Logger) *Handler {
	return &Handler{service: svc, logger: logger}
}

// Routes returns the policy routes, to be mounted by the caller.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.replace)
	r.Patch("/{id}", h.patch)
	r.Delete("/{id}", h.delete)

	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, ok := request.ParsePagination(w, r)
	if !ok {
		return
	}

	policies, err := h.service.List(r.Context(), authn.Actor(r), page.Offset, page.Limit)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	data := make([]PolicyResponse, len(policies.Items))
	for i, policy := range policies.Items {
		data[i] = toPolicyResponse(policy)
	}

	envelope.WriteCollection(w, envelope.NewPagedCollection(data, page.Offset, page.Limit, policies.HasMore))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body CreatePolicyRequest
	if !request.DecodeJSON(w, r, &body) {
		return
	}

	policy, err := h.service.Create(r.Context(), authn.Actor(r), body.toInput())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteCreated(w, path.Join(r.URL.Path, policy.ID.String()), toPolicyResponse(policy))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := policyID(w, r)
	if !ok {
		return
	}

	policy, err := h.service.Get(r.Context(), authn.Actor(r), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toPolicyResponse(policy))
}

func (h *Handler) replace(w http.ResponseWriter, r *http.Request) {
	id, ok := policyID(w, r)
	if !ok {
		return
	}

	var body ReplacePolicyRequest
	if !request.DecodeJSON(w, r, &body) {
		return
	}

	policy, err := h.service.Replace(r.Context(), authn.Actor(r), id, body.toInput())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toPolicyResponse(policy))
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	id, ok := policyID(w, r)
	if !ok {
		return
	}

	var body PatchPolicyRequest
	if !request.DecodeMergePatch(w, r, &body) {
		return
	}

	policy, err := h.service.Patch(r.Context(), authn.Actor(r), id, body.toPatch())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toPolicyResponse(policy))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := policyID(w, r)
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), authn.Actor(r), id); err != nil {
		h.writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// policyID returns the policy ID from the request path, answering one that isn't a UUID like an unknown
// one.
func policyID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return request.UUIDParam(w, r, "id", policyNotFoundProblem)
}
