package repository

import (
	"time"
	"uuid"
)

// SessionRecord is the stored form of a session, one row of the sessions table.
type SessionRecord struct {
	ID         uuid.UUID `db:"id"`
	IdentityID uuid.UUID `db:"identity_id"`
	// TokenHash is the SHA-256 of the session's token.
	TokenHash  []byte    `db:"token_hash"`
	CreatedAt  time.Time `db:"created_at"`
	LastSeenAt time.Time `db:"last_seen_at"`
	ExpiresAt  time.Time `db:"expires_at"`
	// ClientIP is the client's IP address at login, in its text form, or nil if it was unknown.
	ClientIP  *string `db:"client_ip"`
	UserAgent string  `db:"user_agent"`
}
