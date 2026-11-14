package api

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	"pkgsilo/feature/group/service"
)

// CreateGroupRequest is the body of a request that creates a group.
type CreateGroupRequest struct {
	Name string `json:"name"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r CreateGroupRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, service.NameRules...),
	)
}

func (r CreateGroupRequest) toInput() service.GroupInput {
	return service.GroupInput{Name: r.Name}
}

// ReplaceGroupRequest is the body of a request that replaces all settable fields of a group.
type ReplaceGroupRequest struct {
	Name string `json:"name"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r ReplaceGroupRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, service.NameRules...),
	)
}

func (r ReplaceGroupRequest) toInput() service.GroupInput {
	return service.GroupInput{Name: r.Name}
}

// PatchGroupRequest is the body of a request that changes some settable fields of a group, a JSON Merge
// Patch: a member that it leaves out stays unchanged.
type PatchGroupRequest struct {
	Name request.Optional[string] `json:"name"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r PatchGroupRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, request.IfSet(service.NameRules...)),
	)
}

func (r PatchGroupRequest) toPatch() service.GroupPatch {
	return service.GroupPatch{Name: r.Name.Pointer()}
}

// GroupResponse is the representation of a group in responses. A field that the caller may not read is
// absent.
type GroupResponse struct {
	ID        uuid.UUID                 `json:"id"`
	Name      envelope.Field[string]    `json:"name"`
	CreatedBy envelope.Field[uuid.UUID] `json:"createdBy"`
	UpdatedBy envelope.Field[uuid.UUID] `json:"updatedBy"`
	CreatedAt envelope.Field[time.Time] `json:"createdAt"`
	UpdatedAt envelope.Field[time.Time] `json:"updatedAt"`
}

func toGroupResponse(g service.Group) GroupResponse {
	return GroupResponse{
		ID:        g.ID,
		Name:      envelope.NewField(g.Name.Get()),
		CreatedBy: envelope.NewField(g.CreatedBy.Get()),
		UpdatedBy: envelope.NewField(g.UpdatedBy.Get()),
		CreatedAt: envelope.NewField(g.CreatedAt.Get()),
		UpdatedAt: envelope.NewField(g.UpdatedAt.Get()),
	}
}

// MemberResponse is the representation of a group member in responses: the identity and who added
// it when. Reading the members of a group is one action for the whole member list, so no member of it is
// hidden; they are envelope.Fields only because every member of a representation is.
type MemberResponse struct {
	IdentityID envelope.Field[uuid.UUID] `json:"identityId"`
	CreatedBy  envelope.Field[uuid.UUID] `json:"createdBy"`
	CreatedAt  envelope.Field[time.Time] `json:"createdAt"`
}

func toMemberResponse(m service.Member) MemberResponse {
	return MemberResponse{
		IdentityID: envelope.Visible(m.IdentityID),
		CreatedBy:  envelope.Visible(m.CreatedBy),
		CreatedAt:  envelope.Visible(m.CreatedAt),
	}
}
