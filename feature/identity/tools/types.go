// Package tools offers the identity tools of the MCP endpoint, which let an agent find out whom it acts as.
package tools

import (
	"time"
	"uuid"

	"pkgsilo/core/envelope"
	"pkgsilo/feature/identity/service"
)

// WhoamiInput is the input of identity_whoami, which takes no arguments.
type WhoamiInput struct{}

// Validate reports no error: there's nothing to validate.
func (WhoamiInput) Validate() error {
	return nil
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
