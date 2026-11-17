package service

import (
	"net/netip"
	"time"
	"uuid"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// Session keeps a browser logged in as its identity, until it's idle for longer than the idle timeout or
// older than the maximum age. No policy decides who may see it: only its own identity ever does, through the
// session that a request is authenticated with.
type Session struct {
	ID         uuid.UUID
	IdentityID uuid.UUID
	CreatedAt  time.Time
	// LastSeenAt is when the session was last used. It's accurate to about a minute.
	LastSeenAt time.Time
	// ExpiresAt is when the session ends at the latest, however often it's used.
	ExpiresAt time.Time
}

// CreatedSession is a new session with its token, which can't be retrieved later.
type CreatedSession struct {
	Session

	Token string
}

// AuthenticatedSession is the session that a presented token belongs to, as far as a request needs it once
// it's authenticated.
type AuthenticatedSession struct {
	ID uuid.UUID
	// IdentityID is the identity that the request acts as.
	IdentityID uuid.UUID
}

// LoginInput holds what a login sends and what the request tells about its client.
type LoginInput struct {
	Username string
	Password string
	// PresentedToken is the session token that the request already carries, or empty for none. Its session
	// ends, so that a token that someone planted in the browser before the login can't become logged in.
	PresentedToken string
	// ClientIP is the client's IP address, or the zero Addr if it's unknown.
	ClientIP  netip.Addr
	UserAgent string
}

// Lifetimes are the limits of a session's life. A session has expired once either is exceeded.
type Lifetimes struct {
	// IdleTimeout is how long a session lasts without being used.
	IdleTimeout time.Duration
	// MaxAge is how long a session lasts at most after its login, however often it's used.
	MaxAge time.Duration
}

var (
	// UsernameRules are the rules of a login's username: it must not be empty. A username that no identity
	// could have is rejected like an unknown one, so that the login answers both in the same way.
	UsernameRules = []validation.Rule{validation.Required}
	// PasswordRules are the rules of a login's password: it must not be empty. It's never trimmed or
	// normalized, so that it's checked exactly as received.
	PasswordRules = []validation.Rule{validation.Required}
)
