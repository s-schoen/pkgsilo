package service

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/authz"
)

// Password is the password of an identity, as an actor sees it: every field is hidden unless the actor may
// read it. It never holds the password or its hash. The authz tag of a field names its action, such as
// password:read:setAt. Its ID is its identity's, which has no other password.
type Password struct {
	// SetAt is when the password was last set.
	SetAt authz.Field[time.Time] `authz:"setAt"`
}

// PasswordInput holds what a caller sends to set a password.
type PasswordInput struct {
	NewPassword string
	// CurrentPassword is the password that NewPassword replaces, which an actor that may change the password
	// but not reset it must give. It's nil if the caller left it out.
	CurrentPassword *string
}

// NewPasswordRules are the rules of a new password: it must not be empty. There are no other rules, and it's
// never trimmed or normalized, so that it's checked exactly as received.
var NewPasswordRules = []validation.Rule{validation.Required}
