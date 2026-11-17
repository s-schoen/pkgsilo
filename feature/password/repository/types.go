package repository

import (
	"time"
	"uuid"
)

// PasswordRecord is the stored form of a password, one row of the passwords table.
type PasswordRecord struct {
	// IdentityID is the identity that the password belongs to, which has no other password.
	IdentityID uuid.UUID `db:"identity_id"`
	// Hash is the Argon2id hash of the password as a PHC string. The password itself is not stored.
	Hash string `db:"hash"`
	// SetAt is when the password was last set. Rehashing it doesn't change it.
	SetAt time.Time `db:"set_at"`
}

// CredentialsRecord is what checking a username and password needs to know about the identity with that
// username.
type CredentialsRecord struct {
	IdentityID uuid.UUID `db:"identity_id"`
	// Hash is the hash of the identity's password, nil for an identity without a password.
	Hash *string `db:"hash"`
}
