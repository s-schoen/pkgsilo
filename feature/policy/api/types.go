package api

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	"pkgsilo/feature/policy/service"
)

// CreatePolicyRequest is the body of a request that creates a policy.
type CreatePolicyRequest struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r CreatePolicyRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, service.NameRules...),
		validation.Field(&r.Source, service.SourceRules...),
	)
}

func (r CreatePolicyRequest) toInput() service.PolicyInput {
	return service.PolicyInput{Name: r.Name, Source: r.Source}
}

// ReplacePolicyRequest is the body of a request that replaces all settable fields of a policy.
type ReplacePolicyRequest struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r ReplacePolicyRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, service.NameRules...),
		validation.Field(&r.Source, service.SourceRules...),
	)
}

func (r ReplacePolicyRequest) toInput() service.PolicyInput {
	return service.PolicyInput{Name: r.Name, Source: r.Source}
}

// PatchPolicyRequest is the body of a request that changes some settable fields of a policy, a JSON Merge
// Patch: a member that it leaves out stays unchanged.
type PatchPolicyRequest struct {
	Name   request.Optional[string] `json:"name,omitzero"`
	Source request.Optional[string] `json:"source,omitzero"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r PatchPolicyRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, request.IfSet(service.NameRules...)),
		validation.Field(&r.Source, request.IfSet(service.SourceRules...)),
	)
}

func (r PatchPolicyRequest) toPatch() service.PolicyPatch {
	return service.PolicyPatch{Name: r.Name.Pointer(), Source: r.Source.Pointer()}
}

// PolicyResponse is the representation of a policy in responses. A field that the caller may not read is
// absent.
type PolicyResponse struct {
	ID             uuid.UUID                      `json:"id"`
	Name           envelope.Field[string]         `json:"name,omitzero"`
	Source         envelope.Field[string]         `json:"source,omitzero"`
	CreatedBy      envelope.Field[uuid.UUID]      `json:"createdBy,omitzero"`
	UpdatedBy      envelope.Field[uuid.UUID]      `json:"updatedBy,omitzero"`
	CreatedAt      envelope.Field[time.Time]      `json:"createdAt,omitzero"`
	UpdatedAt      envelope.Field[time.Time]      `json:"updatedAt,omitzero"`
	AllowedActions envelope.Field[[]PolicyAction] `json:"allowedActions,omitzero" jsonschema:"The actions that the policies allow the caller to do to it, such as policy:delete or policy:update:source. Absent only from a new policy that the caller may not read."`
}

// PolicyAction is the name of an action that the caller may do to a policy, as PolicyResponse lists
// them: one of service.Actions().Allowable().
type PolicyAction string

// EnumValues returns the names of the actions that PolicyResponse can list, the enum of its schema.
func (PolicyAction) EnumValues() []string {
	return service.Actions().Allowable()
}

func toPolicyResponse(p service.Policy) PolicyResponse {
	return PolicyResponse{
		ID:             p.ID,
		Name:           envelope.NewField(p.Name.Get()),
		Source:         envelope.NewField(p.Source.Get()),
		CreatedBy:      envelope.NewField(p.CreatedBy.Get()),
		UpdatedBy:      envelope.NewField(p.UpdatedBy.Get()),
		CreatedAt:      envelope.NewField(p.CreatedAt.Get()),
		UpdatedAt:      envelope.NewField(p.UpdatedAt.Get()),
		AllowedActions: envelope.NewActions[PolicyAction](p.AllowedActions),
	}
}
