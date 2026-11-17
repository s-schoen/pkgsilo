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
	Username    string `json:"username"    jsonschema:"The unique name of the identity: 3 to 64 lowercase letters and digits, optionally separated by single '.', '_' or '-', such as ada.lovelace."`
	DisplayName string `json:"displayName" jsonschema:"The human-readable name of the identity, such as Ada Lovelace: 1 to 128 characters, without line breaks."`
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
	Username    string `json:"username"    jsonschema:"The unique name of the identity: 3 to 64 lowercase letters and digits, optionally separated by single '.', '_' or '-', such as ada.lovelace."`
	DisplayName string `json:"displayName" jsonschema:"The human-readable name of the identity, such as Ada Lovelace: 1 to 128 characters, without line breaks."`
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
	Username    request.Optional[string] `json:"username,omitzero"    jsonschema:"The new username, with the rules of the username of a new identity."`
	DisplayName request.Optional[string] `json:"displayName,omitzero" jsonschema:"The new display name, with the rules of the display name of a new identity."`
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
	ID          uuid.UUID                 `json:"id"                    jsonschema:"The identity's ID."`
	Username    envelope.Field[string]    `json:"username,omitzero"    jsonschema:"The unique name it is known by."`
	DisplayName envelope.Field[string]    `json:"displayName,omitzero" jsonschema:"Its human-readable name."`
	CreatedBy   envelope.Field[uuid.UUID] `json:"createdBy,omitzero"   jsonschema:"The identity that created it, the nil UUID if pkgsilo did."`
	UpdatedBy   envelope.Field[uuid.UUID] `json:"updatedBy,omitzero"   jsonschema:"The identity that last changed it, the nil UUID if pkgsilo did."`
	CreatedAt   envelope.Field[time.Time] `json:"createdAt,omitzero"   jsonschema:"When it was created."`
	UpdatedAt   envelope.Field[time.Time] `json:"updatedAt,omitzero"   jsonschema:"When it was last changed."`
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
