package api

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

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

// PolicyResponse is the representation of a policy in responses.
type PolicyResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Source    string    `json:"source"`
	CreatedBy uuid.UUID `json:"createdBy"`
	UpdatedBy uuid.UUID `json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func toPolicyResponse(p service.Policy) PolicyResponse {
	return PolicyResponse{
		ID:        p.ID,
		Name:      p.Name,
		Source:    p.Source,
		CreatedBy: p.CreatedBy,
		UpdatedBy: p.UpdatedBy,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}
