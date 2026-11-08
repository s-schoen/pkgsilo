package api

import (
	"regexp"
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	"pkgsilo/feature/identity/service"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)

// Rules of the identity fields, shared by the request bodies that set them.
var (
	usernameRules = []validation.Rule{
		validation.Required,
		validation.Length(3, 64),
		validation.Match(usernamePattern).
			Error("must consist of lowercase letters and digits, optionally separated by single '.', '_' or '-'"),
	}
	displayNameRules = []validation.Rule{
		validation.Required,
		request.NotBlank,
		request.NoControlCharacters,
		validation.RuneLength(1, 128),
	}
)

// CreateIdentityRequest is the body of a request that creates an identity.
type CreateIdentityRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// Validate reports the invalid members of r as validation.Errors.
func (r CreateIdentityRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Username, usernameRules...),
		validation.Field(&r.DisplayName, displayNameRules...),
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
		validation.Field(&r.Username, usernameRules...),
		validation.Field(&r.DisplayName, displayNameRules...),
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
		validation.Field(&r.Username, request.IfSet(usernameRules...)),
		validation.Field(&r.DisplayName, request.IfSet(displayNameRules...)),
	)
}

func (r PatchIdentityRequest) toPatch() service.IdentityPatch {
	return service.IdentityPatch{Username: r.Username.Pointer(), DisplayName: r.DisplayName.Pointer()}
}

// IdentityResponse is the representation of an identity in responses. A field that the caller may not
// read is absent.
type IdentityResponse struct {
	ID          uuid.UUID                 `json:"id"`
	Username    envelope.Field[string]    `json:"username,omitzero"`
	DisplayName envelope.Field[string]    `json:"displayName,omitzero"`
	CreatedBy   envelope.Field[uuid.UUID] `json:"createdBy,omitzero"`
	UpdatedBy   envelope.Field[uuid.UUID] `json:"updatedBy,omitzero"`
	CreatedAt   envelope.Field[time.Time] `json:"createdAt,omitzero"`
	UpdatedAt   envelope.Field[time.Time] `json:"updatedAt,omitzero"`
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
