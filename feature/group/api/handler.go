// Package api serves the group endpoints of the HTTP API.
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
	"pkgsilo/feature/group/service"
)

// Handler serves the group endpoints.
type Handler struct {
	service service.GroupService
	logger  *slog.Logger
}

// NewHandler returns a Handler that manages groups through svc and logs to logger.
func NewHandler(svc service.GroupService, logger *slog.Logger) *Handler {
	return &Handler{service: svc, logger: logger}
}

// Routes returns the group routes, to be mounted by the caller.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.replace)
	r.Patch("/{id}", h.patch)
	r.Delete("/{id}", h.delete)
	r.Get("/{id}/members", h.listMembers)
	r.Put("/{id}/members/{identityID}", h.addMember)
	r.Delete("/{id}/members/{identityID}", h.removeMember)

	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, ok := request.ParsePagination(w, r)
	if !ok {
		return
	}

	groups, err := h.service.List(r.Context(), authn.Actor(r), page.Offset, page.Limit)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	data := make([]GroupResponse, len(groups.Items))
	for i, group := range groups.Items {
		data[i] = toGroupResponse(group)
	}

	envelope.WriteCollection(w, envelope.NewPagedCollection(data, page.Offset, page.Limit, groups.HasMore))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body CreateGroupRequest
	if !request.DecodeJSON(w, r, &body) {
		return
	}

	group, err := h.service.Create(r.Context(), authn.Actor(r), body.toInput())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteCreated(w, path.Join(r.URL.Path, group.ID.String()), toGroupResponse(group))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}

	group, err := h.service.Get(r.Context(), authn.Actor(r), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toGroupResponse(group))
}

func (h *Handler) replace(w http.ResponseWriter, r *http.Request) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}

	var body ReplaceGroupRequest
	if !request.DecodeJSON(w, r, &body) {
		return
	}

	group, err := h.service.Replace(r.Context(), authn.Actor(r), id, body.toInput())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toGroupResponse(group))
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}

	var body PatchGroupRequest
	if !request.DecodeMergePatch(w, r, &body) {
		return
	}

	group, err := h.service.Patch(r.Context(), authn.Actor(r), id, body.toPatch())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	envelope.WriteResource(w, toGroupResponse(group))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), authn.Actor(r), id); err != nil {
		h.writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}

	page, ok := request.ParsePagination(w, r)
	if !ok {
		return
	}

	members, err := h.service.ListMembers(
		r.Context(), authn.Actor(r), id, page.Offset, page.Limit)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	data := make([]MemberResponse, len(members.Items))
	for i, member := range members.Items {
		data[i] = toMemberResponse(member)
	}

	envelope.WriteCollection(w, envelope.NewPagedCollection(data, page.Offset, page.Limit, members.HasMore))
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}

	identityID := memberID(r)

	if !request.EnsureEmptyBody(w, r) {
		return
	}

	if err := h.service.AddMember(r.Context(), authn.Actor(r), id, identityID); err != nil {
		h.writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	id, ok := groupID(w, r)
	if !ok {
		return
	}

	identityID := memberID(r)

	err := h.service.RemoveMember(r.Context(), authn.Actor(r), id, identityID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// memberID returns the identity ID from the path of a member. An ID that isn't a UUID can't identify an
// identity, so it's the Nil UUID, which no identity has: the service then answers it like an unknown
// identity, in the same order of checks.
func memberID(r *http.Request) uuid.UUID {
	identityID, err := uuid.Parse(chi.URLParam(r, "identityID"))
	if err != nil {
		return uuid.Nil()
	}

	return identityID
}

// groupID returns the group ID from the request path, answering one that isn't a UUID like an
// unknown one.
func groupID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return request.UUIDParam(w, r, "id", groupNotFoundProblem)
}
