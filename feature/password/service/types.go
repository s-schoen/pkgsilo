package service

import (
	"time"
	"uuid"

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

// Verified is a username and password that a Verifier accepted.
type Verified struct {
	// IdentityID is the identity with the username, whose password it is.
	IdentityID uuid.UUID
	// PasswordHash is the hash that the identity's password has once it's verified: the stored one, or the
	// rehash that replaced it. What the verification grants, such as a session, must be stored only while the
	// password still has this hash, so that it can't outlast a change of the password that happened during
	// the verification. It must never be logged or returned to a client.
	PasswordHash string
	// RehashErr is why an outdated hash couldn't be replaced with a rehash, or nil. The password is verified
	// all the same, and the next verification tries the rehash again.
	RehashErr error
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
