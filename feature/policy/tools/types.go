// Package tools offers the policy tools of the MCP endpoint, which let an agent read the Cedar schema and
// write, fix and remove the policies that decide what identities may do in pkgsilo.
package tools

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
	"pkgsilo/feature/policy/service"
)

// GetSchemaInput is the input of policy_get_schema, which takes no arguments.
type GetSchemaInput struct{}

// Validate reports no error: there's nothing to validate.
func (GetSchemaInput) Validate() error {
	return nil
}

// GetInput is the input of policy_get.
type GetInput struct {
	ID uuid.UUID `json:"id" jsonschema:"The ID of the policy."`
}

// Validate reports no error: any ID can be looked up.
func (GetInput) Validate() error {
	return nil
}

// CreateInput is the input of policy_create. Its members are named like the members of CreatePolicyRequest
// in the HTTP API and the fields of the policy:create actions.
type CreateInput struct {
	Name   string `json:"name"   jsonschema:"The unique name of the new policy, such as maintainers-read-everything: 1 to 128 characters, without line breaks."`
	Source string `json:"source" jsonschema:"The Cedar statements of the policy, with the rules in the tool's description, such as permit (principal in Group::\"<group ID>\", action in Action::\"read\", resource);"`
}

// Validate reports the invalid members of in as validation.Errors, with the rules of the service. The
// service checks the source itself.
func (in CreateInput) Validate() error {
	return validation.ValidateStruct(&in,
		validation.Field(&in.Name, service.NameRules...),
		validation.Field(&in.Source, service.SourceRules...),
	)
}

func (in CreateInput) toInput() service.PolicyInput {
	return service.PolicyInput{Name: in.Name, Source: in.Source}
}

// UpdateInput is the input of policy_update, which changes the members that it sets and keeps the others.
// Its members are named like the members of PatchPolicyRequest in the HTTP API and the fields of the
// policy:update actions. Like a member of the patch, a member can't be null.
type UpdateInput struct {
	ID     uuid.UUID `json:"id"               jsonschema:"The ID of the policy to change."`
	Name   *string   `json:"name,omitempty"   jsonschema:"The new name, with the rules of policy_create. Leave it out to keep the current one."`
	Source *string   `json:"source,omitempty" jsonschema:"The new Cedar statements, which replace all current ones, with the rules of policy_create. Leave it out to keep the current ones."`
}

// Validate reports the invalid members of in that it sets as validation.Errors, with the rules of the
// service.
func (in UpdateInput) Validate() error {
	return validation.ValidateStruct(&in,
		validation.Field(&in.Name, validation.When(in.Name != nil, service.NameRules...)),
		validation.Field(&in.Source, validation.When(in.Source != nil, service.SourceRules...)),
	)
}

func (in UpdateInput) toPatch() service.PolicyPatch {
	return service.PolicyPatch{Name: in.Name, Source: in.Source}
}

// DeleteInput is the input of policy_delete.
type DeleteInput struct {
	ID uuid.UUID `json:"id" jsonschema:"The ID of the policy to delete."`
}

// Validate reports no error: any ID can be looked up.
func (DeleteInput) Validate() error {
	return nil
}

// SchemaOutput is the output of policy_get_schema. The schema is never hidden; it's an envelope.Field only
// because every member of an output is.
type SchemaOutput struct {
	Schema envelope.Field[string] `json:"schema,omitzero" jsonschema:"The Cedar schema, in Cedar's human-readable schema format."`
}

// PolicyOutput is a policy in the output of a tool, as PolicyResponse is in the HTTP API. A field that the
// actor may not read is absent. The members' descriptions are part of the output schema, which describes
// them to agents.
type PolicyOutput struct {
	ID        uuid.UUID                 `json:"id" jsonschema:"The policy's ID."`
	Name      envelope.Field[string]    `json:"name,omitzero" jsonschema:"The unique name it is known by."`
	Source    envelope.Field[string]    `json:"source,omitzero" jsonschema:"Its Cedar statements."`
	CreatedBy envelope.Field[uuid.UUID] `json:"createdBy,omitzero" jsonschema:"The identity that created it."`
	UpdatedBy envelope.Field[uuid.UUID] `json:"updatedBy,omitzero" jsonschema:"The identity that last changed it."`
	CreatedAt envelope.Field[time.Time] `json:"createdAt,omitzero" jsonschema:"When it was created."`
	UpdatedAt envelope.Field[time.Time] `json:"updatedAt,omitzero" jsonschema:"When it was last changed."`
}

func toPolicyOutput(p service.Policy) PolicyOutput {
	return PolicyOutput{
		ID:        p.ID,
		Name:      envelope.NewField(p.Name.Get()),
		Source:    envelope.NewField(p.Source.Get()),
		CreatedBy: envelope.NewField(p.CreatedBy.Get()),
		UpdatedBy: envelope.NewField(p.UpdatedBy.Get()),
		CreatedAt: envelope.NewField(p.CreatedAt.Get()),
		UpdatedAt: envelope.NewField(p.UpdatedAt.Get()),
	}
}
