package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"pkgsilo/core/authn"
	"pkgsilo/core/middleware"
	"pkgsilo/feature/apikey/service"
)

// keyHeader is the header that holds a request's API key: X-API-Key, in the canonical form that
// net/http stores it in. Header names are case-insensitive, so clients can send either.
const keyHeader = "X-Api-Key"

// Authenticator authenticates requests by the API key in their X-API-Key header.
type Authenticator struct {
	service service.APIKeyService
}

var _ authn.Authenticator = (*Authenticator)(nil)

// NewAuthenticator returns an Authenticator that checks API keys through svc.
func NewAuthenticator(svc service.APIKeyService) *Authenticator {
	return &Authenticator{service: svc}
}

// Method returns the name of the authentication method.
func (*Authenticator) Method() string {
	return "api_key"
}

// Challenge returns the challenge for the WWW-Authenticate header. No scheme is registered for API
// key headers, so it only tells people which method is meant.
func (*Authenticator) Challenge() string {
	return `ApiKey realm="pkgsilo"`
}

// Authenticate returns the principal of the API key in the request's header, and adds the API key's
// ID to the request's logs, so that the requests of a leaked key can be told from the identity's others.
func (a *Authenticator) Authenticate(r *http.Request) (authn.Principal, error) {
	presented := r.Header.Get(keyHeader)
	if presented == "" {
		return authn.Principal{}, authn.ErrNoCredentials
	}

	key, err := a.service.Authenticate(r.Context(), presented)
	switch {
	case err == nil:
		middleware.AddLogAttrs(r.Context(), slog.String("api_key_id", key.ID.String()))

		return authn.Principal{IdentityID: key.IdentityID}, nil
	case errors.Is(err, service.ErrMalformedKey):
		return authn.Principal{}, fmt.Errorf("%w: %w", authn.ErrMalformedCredentials, err)
	case errors.Is(err, service.ErrUnknownKey):
		return authn.Principal{}, fmt.Errorf("%w: %w", authn.ErrUnknownCredentials, err)
	case errors.Is(err, service.ErrExpiredKey):
		return authn.Principal{}, fmt.Errorf("%w: %w", authn.ErrExpiredCredentials, err)
	default:
		return authn.Principal{}, fmt.Errorf("authenticate api key: %w", err)
	}
}
