// Package tools offers the identity tools of the MCP endpoint, which let an agent find and manage the
// identities of pkgsilo and find out whom it acts as.
package tools

import (
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
	"pkgsilo/feature/identity/service"
)

// WhoamiInput is the input of identity_whoami, which takes no arguments.
type WhoamiInput struct{}

// Validate reports no error: there's nothing to validate.
func (WhoamiInput) Validate() error {
	return nil
}

// GetInput is the input of identity_get.
type GetInput struct {
	ID uuid.UUID `json:"id" jsonschema:"The ID of the identity."`
}

// Validate reports no error: any ID can be looked up.
func (GetInput) Validate() error {
	return nil
}

// CreateInput is the input of identity_create. Its members are named like the members of
// CreateIdentityRequest in the HTTP API and the fields of the identity:create actions.
type CreateInput struct {
	Username    string `json:"username"    jsonschema:"The unique name of the new identity: 3 to 64 lowercase letters and digits, optionally separated by single '.', '_' or '-', such as ada.lovelace."`
	DisplayName string `json:"displayName" jsonschema:"The human-readable name of the new identity, such as Ada Lovelace: 1 to 128 characters, without line breaks."`
}

// Validate reports the invalid members of in as validation.Errors, with the rules of the service.
func (in CreateInput) Validate() error {
	return validation.ValidateStruct(&in,
		validation.Field(&in.Username, service.UsernameRules...),
		validation.Field(&in.DisplayName, service.DisplayNameRules...),
	)
}

func (in CreateInput) toInput() service.IdentityInput {
	return service.IdentityInput{Username: in.Username, DisplayName: in.DisplayName}
}

// UpdateInput is the input of identity_update, which changes the members that it sets and keeps the others.
// Its members are named like the members of PatchIdentityRequest in the HTTP API and the fields of the
// identity:update actions.
type UpdateInput struct {
	ID          uuid.UUID `json:"id"                    jsonschema:"The ID of the identity to change."`
	Username    *string   `json:"username,omitempty"    jsonschema:"The new username, with the rules of identity_create. Leave it out to keep the current one."`
	DisplayName *string   `json:"displayName,omitempty" jsonschema:"The new display name, with the rules of identity_create. Leave it out to keep the current one."`
}

// Validate reports the invalid members of in that it sets as validation.Errors, with the rules of the
// service.
func (in UpdateInput) Validate() error {
	return validation.ValidateStruct(&in,
		validation.Field(&in.Username, validation.When(in.Username != nil, service.UsernameRules...)),
		validation.Field(&in.DisplayName, validation.When(in.DisplayName != nil, service.DisplayNameRules...)),
	)
}

func (in UpdateInput) toPatch() service.IdentityPatch {
	return service.IdentityPatch{Username: in.Username, DisplayName: in.DisplayName}
}

// IdentityOutput is an identity in the output of a tool, as IdentityResponse is in the HTTP API. A field
// that the actor may not read is absent. The members' descriptions are part of the output schema, which
// describes them to agents.
type IdentityOutput struct {
	ID          uuid.UUID                 `json:"id" jsonschema:"The identity's ID."`
	Username    envelope.Field[string]    `json:"username,omitzero" jsonschema:"The unique name it is known by."`
	DisplayName envelope.Field[string]    `json:"displayName,omitzero" jsonschema:"Its human-readable name."`
	CreatedBy   envelope.Field[uuid.UUID] `json:"createdBy,omitzero" jsonschema:"The identity that created it."`
	UpdatedBy   envelope.Field[uuid.UUID] `json:"updatedBy,omitzero" jsonschema:"The identity that last changed it."`
	CreatedAt   envelope.Field[time.Time] `json:"createdAt,omitzero" jsonschema:"When it was created."`
	UpdatedAt   envelope.Field[time.Time] `json:"updatedAt,omitzero" jsonschema:"When it was last changed."`
}

func toIdentityOutput(i service.Identity) IdentityOutput {
	return IdentityOutput{
		ID:          i.ID,
		Username:    envelope.NewField(i.Username.Get()),
		DisplayName: envelope.NewField(i.DisplayName.Get()),
		CreatedBy:   envelope.NewField(i.CreatedBy.Get()),
		UpdatedBy:   envelope.NewField(i.UpdatedBy.Get()),
		CreatedAt:   envelope.NewField(i.CreatedAt.Get()),
		UpdatedAt:   envelope.NewField(i.UpdatedAt.Get()),
	}
}
