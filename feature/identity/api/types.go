package api

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	"pkgsilo/feature/identity/service"
)

// CreateIdentityRequest is the body of a request that creates an identity.
type CreateIdentityRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r CreateIdentityRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Username, service.UsernameRules...),
		validation.Field(&r.DisplayName, service.DisplayNameRules...),
	)
}

func (r CreateIdentityRequest) toInput() service.IdentityInput {
	return service.IdentityInput{Username: r.Username, DisplayName: r.DisplayName}
}

// ReplaceIdentityRequest is the body of a request that replaces all settable fields of an identity.
type ReplaceIdentityRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r ReplaceIdentityRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Username, service.UsernameRules...),
		validation.Field(&r.DisplayName, service.DisplayNameRules...),
	)
}

func (r ReplaceIdentityRequest) toInput() service.IdentityInput {
	return service.IdentityInput{Username: r.Username, DisplayName: r.DisplayName}
}

// PatchIdentityRequest is the body of a request that changes some settable fields of an identity, a JSON
// Merge Patch: a member that it leaves out stays unchanged.
type PatchIdentityRequest struct {
	Username    request.Optional[string] `json:"username"`
	DisplayName request.Optional[string] `json:"displayName"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r PatchIdentityRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Username, request.IfSet(service.UsernameRules...)),
		validation.Field(&r.DisplayName, request.IfSet(service.DisplayNameRules...)),
	)
}

func (r PatchIdentityRequest) toPatch() service.IdentityPatch {
	return service.IdentityPatch{Username: r.Username.Pointer(), DisplayName: r.DisplayName.Pointer()}
}

// IdentityResponse is the representation of an identity in responses. A field that the caller may not
// read is absent.
type IdentityResponse struct {
	ID          uuid.UUID                 `json:"id"`
	Username    envelope.Field[string]    `json:"username"`
	DisplayName envelope.Field[string]    `json:"displayName"`
	CreatedBy   envelope.Field[uuid.UUID] `json:"createdBy"`
	UpdatedBy   envelope.Field[uuid.UUID] `json:"updatedBy"`
	CreatedAt   envelope.Field[time.Time] `json:"createdAt"`
	UpdatedAt   envelope.Field[time.Time] `json:"updatedAt"`
}

func toIdentityResponse(i service.Identity) IdentityResponse {
	return IdentityResponse{
		ID:          i.ID,
		Username:    envelope.NewField(i.Username.Get()),
		DisplayName: envelope.NewField(i.DisplayName.Get()),
		CreatedBy:   envelope.NewField(i.CreatedBy.Get()),
		UpdatedBy:   envelope.NewField(i.UpdatedBy.Get()),
		CreatedAt:   envelope.NewField(i.CreatedAt.Get()),
		UpdatedAt:   envelope.NewField(i.UpdatedAt.Get()),
	}
}
