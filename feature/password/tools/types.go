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
	SetAt envelope.Field[time.Time] `json:"setAt,omitzero" jsonschema:"When the password was last set."`
}

func toPasswordOutput(p service.Password) PasswordOutput {
	return PasswordOutput{SetAt: envelope.NewField(p.SetAt.Get())}
}
