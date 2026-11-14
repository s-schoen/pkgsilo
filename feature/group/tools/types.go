// Package tools offers the group tools of the MCP endpoint, which let an agent manage the groups of
// pkgsilo and their members, and so grant and revoke the access that policies give to groups.
package tools

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/group/service"
)

// GetInput is the input of group_get.
type GetInput struct {
	ID uuid.UUID `json:"id" jsonschema:"The ID of the group."`
}

// Validate reports no error: any ID can be looked up.
func (GetInput) Validate() error {
	return nil
}

// CreateInput is the input of group_create. Its members are named like the members of CreateGroupRequest
// in the HTTP API and the fields of the group:create actions.
type CreateInput struct {
	Name string `json:"name" jsonschema:"The unique name of the new group, such as maintainers: 1 to 128 characters, without line breaks."`
}

// Validate reports the invalid members of in as validation.Errors, with the rules of the service.
func (in CreateInput) Validate() error {
	return validation.ValidateStruct(&in,
		validation.Field(&in.Name, service.NameRules...),
	)
}

func (in CreateInput) toInput() service.GroupInput {
	return service.GroupInput{Name: in.Name}
}

// UpdateInput is the input of group_update, which changes the members that it sets and keeps the others.
// Its members are named like the members of PatchGroupRequest in the HTTP API and the fields of the
// group:update actions.
type UpdateInput struct {
	ID   uuid.UUID `json:"id"             jsonschema:"The ID of the group to change."`
	Name *string   `json:"name,omitempty" jsonschema:"The new name, with the rules of group_create. Leave it out to keep the current one."`
}

// Validate reports the invalid members of in that it sets as validation.Errors, with the rules of the
// service.
func (in UpdateInput) Validate() error {
	return validation.ValidateStruct(&in,
		validation.Field(&in.Name, validation.When(in.Name != nil, service.NameRules...)),
	)
}

func (in UpdateInput) toPatch() service.GroupPatch {
	return service.GroupPatch{Name: in.Name}
}

// ListMembersInput is the input of group_list_members: the group, and the page of its members. Its
// Validate is ListInput's, since any group ID can be looked up.
type ListMembersInput struct {
	mcpserver.ListInput

	GroupID uuid.UUID `json:"groupId" jsonschema:"The ID of the group whose members to list."`
}

// MemberInput is the input of group_add_member and group_remove_member: a group and an identity.
type MemberInput struct {
	GroupID    uuid.UUID `json:"groupId"    jsonschema:"The ID of the group."`
	IdentityID uuid.UUID `json:"identityId" jsonschema:"The ID of the identity, as identity_list and identity_get return it."`
}

// Validate reports no error: any IDs can be looked up.
func (MemberInput) Validate() error {
	return nil
}

// GroupOutput is a group in the output of a tool, as GroupResponse is in the HTTP API. A field that the
// actor may not read is absent. The members' descriptions are part of the output schema, which describes
// them to agents.
type GroupOutput struct {
	ID        uuid.UUID                 `json:"id" jsonschema:"The group's ID."`
	Name      envelope.Field[string]    `json:"name,omitzero" jsonschema:"The unique name it is known by."`
	CreatedBy envelope.Field[uuid.UUID] `json:"createdBy,omitzero" jsonschema:"The identity that created it."`
	UpdatedBy envelope.Field[uuid.UUID] `json:"updatedBy,omitzero" jsonschema:"The identity that last changed it."`
	CreatedAt envelope.Field[time.Time] `json:"createdAt,omitzero" jsonschema:"When it was created."`
	UpdatedAt envelope.Field[time.Time] `json:"updatedAt,omitzero" jsonschema:"When it was last changed."`
}

func toGroupOutput(g service.Group) GroupOutput {
	return GroupOutput{
		ID:        g.ID,
		Name:      envelope.NewField(g.Name.Get()),
		CreatedBy: envelope.NewField(g.CreatedBy.Get()),
		UpdatedBy: envelope.NewField(g.UpdatedBy.Get()),
		CreatedAt: envelope.NewField(g.CreatedAt.Get()),
		UpdatedAt: envelope.NewField(g.UpdatedAt.Get()),
	}
}

// MemberOutput is a member of a group in the output of group_list_members, as MemberResponse is in the
// HTTP API: the identity and who added it when. Reading the members of a group is one action for the whole
// member list, so no member of it is hidden; they are envelope.Fields only because every member of an
// output is.
type MemberOutput struct {
	IdentityID envelope.Field[uuid.UUID] `json:"identityId,omitzero" jsonschema:"The ID of the member, an identity, which identity_get returns."`
	CreatedBy  envelope.Field[uuid.UUID] `json:"createdBy,omitzero" jsonschema:"The identity that added it to the group."`
	CreatedAt  envelope.Field[time.Time] `json:"createdAt,omitzero" jsonschema:"When it was added to the group."`
}

func toMemberOutput(m service.Member) MemberOutput {
	return MemberOutput{
		IdentityID: envelope.Visible(m.IdentityID),
		CreatedBy:  envelope.Visible(m.CreatedBy),
		CreatedAt:  envelope.Visible(m.CreatedAt),
	}
}
