package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"pkgsilo/core/authn"
	"pkgsilo/core/middleware"
	"pkgsilo/feature/session/service"
)

// method is the name of the authentication method of sessions, which Handler checks to find the session of a
// request.
const method = "session"

// Authenticator authenticates requests by the session token in their session cookie. It's meant for the
// routes of the API only, since only browsers send the cookie: an agent on the MCP endpoint uses an API key.
type Authenticator struct {
	service service.SessionService
}

var _ authn.Authenticator = (*Authenticator)(nil)

// NewAuthenticator returns an Authenticator that checks session tokens through svc.
func NewAuthenticator(svc service.SessionService) *Authenticator {
	return &Authenticator{service: svc}
}

// Method returns the name of the authentication method.
func (*Authenticator) Method() string {
	return method
}

// Challenge returns no challenge, since cookies have no scheme for the WWW-Authenticate header.
func (*Authenticator) Challenge() string {
	return ""
}

// Authenticate returns the principal of the session of the request's cookie, with the session's ID as its
// credential, and adds the session's ID to the request's logs.
func (a *Authenticator) Authenticate(r *http.Request) (authn.Principal, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return authn.Principal{}, authn.ErrNoCredentials
	}

	session, err := a.service.Authenticate(r.Context(), cookie.Value)
	switch {
	case err == nil:
		middleware.AddLogAttrs(r.Context(), slog.String("session_id", session.ID.String()))

		return authn.Principal{IdentityID: session.IdentityID, CredentialID: session.ID}, nil
	case errors.Is(err, service.ErrMalformedToken):
		return authn.Principal{}, fmt.Errorf("%w: %w", authn.ErrMalformedCredentials, err)
	case errors.Is(err, service.ErrUnknownToken):
		return authn.Principal{}, fmt.Errorf("%w: %w", authn.ErrUnknownCredentials, err)
	case errors.Is(err, service.ErrExpiredSession):
		return authn.Principal{}, fmt.Errorf("%w: %w", authn.ErrExpiredCredentials, err)
	default:
		return authn.Principal{}, fmt.Errorf("authenticate session: %w", err)
	}
}
