// Package tools offers the API key tools of the MCP endpoint, which let an agent review the API keys of
// pkgsilo's identities and revoke them. It offers no tool that creates an API key, since the new key would
// pass through the model's context.
package tools

import (
	"time"
	"uuid"

	"pkgsilo/core/envelope"
	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/apikey/service"
)

// ListInput is the input of apikey_list: the identity, and the page of its API keys. Its Validate is
// mcpserver.ListInput's, since any identity ID can be looked up.
type ListInput struct {
	mcpserver.ListInput

	IdentityID uuid.UUID `json:"identityId" jsonschema:"The ID of the identity whose API keys to list, as identity_list and identity_get return it."`
}

// KeyInput is the input of apikey_get and apikey_delete: an API key of an identity.
type KeyInput struct {
	IdentityID uuid.UUID `json:"identityId" jsonschema:"The ID of the identity that the API key belongs to, as identity_list and identity_get return it."`
	ID         uuid.UUID `json:"id"         jsonschema:"The ID of the API key, as apikey_list returns it."`
}

// Validate reports no error: any IDs can be looked up.
func (KeyInput) Validate() error {
	return nil
}

// APIKeyOutput is an API key in the output of a tool, as KeyResponse is in the HTTP API. It never holds the
// key itself. A field that the actor may not read is absent. The members' descriptions are part of the
// output schema, which describes them to agents.
type APIKeyOutput struct {
	ID             uuid.UUID                      `json:"id" jsonschema:"The API key's ID."`
	IdentityID     envelope.Field[uuid.UUID]      `json:"identityId,omitzero" jsonschema:"The identity that it authenticates as."`
	Name           envelope.Field[string]         `json:"name,omitzero" jsonschema:"The name that tells its holder what it is for, such as ci; not unique."`
	KeySuffix      envelope.Field[string]         `json:"keySuffix,omitzero" jsonschema:"The last characters of its key, which tell the holder of a key which API key it is."`
	ExpiresAt      envelope.Field[*time.Time]     `json:"expiresAt,omitzero" jsonschema:"When it stops working, or null if it never expires."`
	LastUsedAt     envelope.Field[*time.Time]     `json:"lastUsedAt,omitzero" jsonschema:"When it last authenticated a request, accurate to about a minute, or null if it has never been used."`
	CreatedBy      envelope.Field[uuid.UUID]      `json:"createdBy,omitzero" jsonschema:"The identity that created it."`
	CreatedAt      envelope.Field[time.Time]      `json:"createdAt,omitzero" jsonschema:"When it was created."`
	AllowedActions envelope.Field[[]APIKeyAction] `json:"allowedActions,omitzero" jsonschema:"The actions that the policies allow you to do to it: apikey:delete to delete it with apikey_delete. Absent only from a new API key that you may not read."`
}

// APIKeyAction is the name of an action that the actor may do to an API key, as APIKeyOutput lists
// them: one of service.Actions().Allowable().
type APIKeyAction string

// EnumValues returns the names of the actions that APIKeyOutput can list, the enum of its schema.
func (APIKeyAction) EnumValues() []string {
	return service.Actions().Allowable()
}

func toAPIKeyOutput(k service.APIKey) APIKeyOutput {
	return APIKeyOutput{
		ID:             k.ID,
		IdentityID:     envelope.NewField(k.IdentityID.Get()),
		Name:           envelope.NewField(k.Name.Get()),
		KeySuffix:      envelope.NewField(k.KeySuffix.Get()),
		ExpiresAt:      envelope.NewField(k.ExpiresAt.Get()),
		LastUsedAt:     envelope.NewField(k.LastUsedAt.Get()),
		CreatedBy:      envelope.NewField(k.CreatedBy.Get()),
		CreatedAt:      envelope.NewField(k.CreatedAt.Get()),
		AllowedActions: envelope.NewActions[APIKeyAction](k.AllowedActions),
	}
}
