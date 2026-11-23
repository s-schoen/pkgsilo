package api

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
	"pkgsilo/feature/password/service"
)

// SetPasswordRequest is the body of a request that sets a password.
type SetPasswordRequest struct {
	NewPassword string `json:"newPassword" jsonschema:"The new password. It must not be empty; there are no other rules, and it's used exactly as sent."`
	// CurrentPassword is nil if the request leaves it out.
	CurrentPassword *string `json:"currentPassword,omitzero" jsonschema:"The password that the new one replaces. Required unless the policies allow the caller to reset the password, in which case it's ignored."`
}

// Validate reports the invalid members of r as validation.Errors.
func (r SetPasswordRequest) Validate() error {
	return validation.ValidateStruct(&r, validation.Field(&r.NewPassword, service.NewPasswordRules...))
}

func (r SetPasswordRequest) toInput() service.PasswordInput {
	return service.PasswordInput{NewPassword: r.NewPassword, CurrentPassword: r.CurrentPassword}
}

// PasswordResponse is the representation of a password in responses. It never holds the password or its
// hash. A field that the caller may not read is absent.
type PasswordResponse struct {
	SetAt          envelope.Field[time.Time]        `json:"setAt,omitzero" jsonschema:"When the password was last set."`
	AllowedActions envelope.Field[[]PasswordAction] `json:"allowedActions,omitzero" jsonschema:"The actions that the policies allow the caller to do to it, such as password:reset to set it without the current one, or password:change to set it with the current one. Absent only from a password that the caller set but may not read."`
}

// PasswordAction is the name of an action that the caller may do to a password, as PasswordResponse lists
// them: one of service.Actions().Allowable().
type PasswordAction string

// EnumValues returns the names of the actions that PasswordResponse can list, the enum of its schema.
func (PasswordAction) EnumValues() []string {
	return service.Actions().Allowable()
}

func toPasswordResponse(p service.Password) PasswordResponse {
	return PasswordResponse{
		SetAt:          envelope.NewField(p.SetAt.Get()),
		AllowedActions: envelope.NewActions[PasswordAction](p.AllowedActions),
	}
}
