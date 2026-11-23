// Package tools offers the password tools of the MCP endpoint, which let an agent see when the passwords of
// pkgsilo's identities were set and delete them. No tool accepts a password, since a plaintext password
// would pass through the model's context and logs: passwords are set through the REST API only.
package tools

import (
	"time"
	"uuid"

	"pkgsilo/core/envelope"
	"pkgsilo/feature/password/service"
)

// PasswordInput is the input of password_get and password_delete: the identity whose password it is.
type PasswordInput struct {
	IdentityID uuid.UUID `json:"identityId" jsonschema:"The ID of the identity whose password it is, as identity_list and identity_get return it."`
}

// Validate reports no error: any identity ID can be looked up.
func (PasswordInput) Validate() error {
	return nil
}

// PasswordOutput is a password in the output of a tool, as PasswordResponse is in the HTTP API. It never
// holds the password or its hash. A field that the actor may not read is absent. The members' descriptions
// are part of the output schema, which describes them to agents.
type PasswordOutput struct {
	SetAt          envelope.Field[time.Time]        `json:"setAt,omitzero" jsonschema:"When the password was last set."`
	AllowedActions envelope.Field[[]PasswordAction] `json:"allowedActions,omitzero" jsonschema:"The actions that the policies allow you to do to it, such as password:reset to set it without the current one, or password:change to set it with the current one. Absent only from a password that you set but may not read."`
}

// PasswordAction is the name of an action that the actor may do to a password, as PasswordOutput lists
// them: one of service.Actions().Allowable().
type PasswordAction string

// EnumValues returns the names of the actions that PasswordOutput can list, the enum of its schema.
func (PasswordAction) EnumValues() []string {
	return service.Actions().Allowable()
}

func toPasswordOutput(p service.Password) PasswordOutput {
	return PasswordOutput{
		SetAt:          envelope.NewField(p.SetAt.Get()),
		AllowedActions: envelope.NewActions[PasswordAction](p.AllowedActions),
	}
}
