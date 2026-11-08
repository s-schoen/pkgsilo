package api

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	"pkgsilo/feature/policy/service"
)

// Rules of the policy fields, shared by the request bodies that set them. The service checks the source
// itself, and reports a mistake with an invalid-policy problem.
var (
	nameRules = []validation.Rule{
		validation.Required,
		request.NotBlank,
		request.NoControlCharacters,
		validation.RuneLength(1, 128),
	}
	sourceRules = []validation.Rule{
		validation.Required,
	}
)

// CreatePolicyRequest is the body of a request that creates a policy.
type CreatePolicyRequest struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r CreatePolicyRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, nameRules...),
		validation.Field(&r.Source, sourceRules...),
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
		validation.Field(&r.Name, nameRules...),
		validation.Field(&r.Source, sourceRules...),
	)
}

func (r ReplacePolicyRequest) toInput() service.PolicyInput {
	return service.PolicyInput{Name: r.Name, Source: r.Source}
}

// PatchPolicyRequest is the body of a request that changes some settable fields of a policy, a JSON Merge
// Patch: a member that it leaves out stays unchanged.
type PatchPolicyRequest struct {
	Name   request.Optional[string] `json:"name"`
	Source request.Optional[string] `json:"source"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r PatchPolicyRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name, request.IfSet(nameRules...)),
		validation.Field(&r.Source, request.IfSet(sourceRules...)),
	)
}

func (r PatchPolicyRequest) toPatch() service.PolicyPatch {
	return service.PolicyPatch{Name: r.Name.Pointer(), Source: r.Source.Pointer()}
}

// PolicyResponse is the representation of a policy in responses. A field that the caller may not read is
// absent.
type PolicyResponse struct {
	ID        uuid.UUID                 `json:"id"`
	Name      envelope.Field[string]    `json:"name"`
	Source    envelope.Field[string]    `json:"source"`
	CreatedBy envelope.Field[uuid.UUID] `json:"createdBy"`
	UpdatedBy envelope.Field[uuid.UUID] `json:"updatedBy"`
	CreatedAt envelope.Field[time.Time] `json:"createdAt"`
	UpdatedAt envelope.Field[time.Time] `json:"updatedAt"`
}

func toPolicyResponse(p service.Policy) PolicyResponse {
	return PolicyResponse{
		ID:        p.ID,
		Name:      envelope.NewField(p.Name.Get()),
		Source:    envelope.NewField(p.Source.Get()),
		CreatedBy: envelope.NewField(p.CreatedBy.Get()),
		UpdatedBy: envelope.NewField(p.UpdatedBy.Get()),
		CreatedAt: envelope.NewField(p.CreatedAt.Get()),
		UpdatedAt: envelope.NewField(p.UpdatedAt.Get()),
	}
}
