package api

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/group/service"
)

// nameRules are the rules of the group name, shared by the request bodies that set it.
var nameRules = []validation.Rule{
	validation.Required,
	validation.RuneLength(1, 128),
}

// CreateGroupRequest is the body of a request that creates a group.
type CreateGroupRequest struct {
	Name string `json:"name"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r CreateGroupRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, nameRules...),
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
		validation.Field(&r.Name, nameRules...),
	)
}

func (r ReplaceGroupRequest) toInput() service.GroupInput {
	return service.GroupInput{Name: r.Name}
}

// GroupResponse is the representation of a group in responses.
type GroupResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedBy uuid.UUID `json:"createdBy"`
	UpdatedBy uuid.UUID `json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func toGroupResponse(g service.Group) GroupResponse {
	return GroupResponse{
		ID:        g.ID,
		Name:      g.Name,
		CreatedBy: g.CreatedBy,
		UpdatedBy: g.UpdatedBy,
		CreatedAt: g.CreatedAt,
		UpdatedAt: g.UpdatedAt,
	}
}

// MemberResponse is the representation of a group member in responses: the identity and who added
// it when.
type MemberResponse struct {
	IdentityID uuid.UUID `json:"identityId"`
	CreatedBy  uuid.UUID `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
}

func toMemberResponse(m service.Member) MemberResponse {
	return MemberResponse{
		IdentityID: m.IdentityID,
		CreatedBy:  m.CreatedBy,
		CreatedAt:  m.CreatedAt,
	}
}
