package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/authn"
	"pkgsilo/core/router"
	"pkgsilo/feature/session/service"
	"pkgsilo/feature/session/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
)

const presentedToken = "pss_PRESENTEDTOKEN"

// authenticatedSession is ada's session as Authenticate returns it.
var authenticatedSession = service.AuthenticatedSession{ID: sessionID, IdentityID: apitest.Actor}

// requestWithCookie returns a request that carries the session cookie with presentedToken.
func requestWithCookie(t *testing.T) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-session", Value: presentedToken})

	return req
}

func TestAuthenticator_Method(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "session", NewAuthenticator(mocks.NewMockSessionService(t)).Method())
}

func TestAuthenticator_Challenge(t *testing.T) {
	t.Parallel()

	assert.Empty(t, NewAuthenticator(mocks.NewMockSessionService(t)).Challenge(), "cookies have no scheme")
}

func TestAuthenticator_Authenticate(t *testing.T) {
	t.Parallel()
	svc := mocks.NewMockSessionService(t)
	svc.EXPECT().Authenticate(mock.Anything, presentedToken).Return(authenticatedSession, nil)

	got, err := NewAuthenticator(svc).Authenticate(requestWithCookie(t))
	require.NoError(t, err)

	assert.Equal(t, authn.Principal{IdentityID: apitest.Actor, CredentialID: sessionID}, got)
}

func TestAuthenticator_Authenticate_NoCredentials(t *testing.T) {
	t.Parallel()
	// The mock fails the test on any call.
	authenticator := NewAuthenticator(mocks.NewMockSessionService(t))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: presentedToken})

	_, err := authenticator.Authenticate(req)

	assert.ErrorIs(t, err, authn.ErrNoCredentials)
}

func TestAuthenticator_Authenticate_Rejected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cause  error
		reason error
	}{
		{service.ErrMalformedToken, authn.ErrMalformedCredentials},
		{service.ErrUnknownToken, authn.ErrUnknownCredentials},
		{service.ErrExpiredSession, authn.ErrExpiredCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.cause.Error(), func(t *testing.T) {
			t.Parallel()
			svc := mocks.NewMockSessionService(t)
			svc.EXPECT().Authenticate(mock.Anything, presentedToken).
				Return(service.AuthenticatedSession{}, fmt.Errorf("authenticate: %w", tt.cause))

			_, err := NewAuthenticator(svc).Authenticate(requestWithCookie(t))

			require.ErrorIs(t, err, tt.reason)
			require.ErrorIs(t, err, authn.ErrInvalidCredentials)
			assert.ErrorContains(t, err, tt.cause.Error(), "the logs must tell the cause")
		})
	}
}

func TestAuthenticator_Authenticate_UnexpectedError(t *testing.T) {
	t.Parallel()
	errDB := errors.New("connection refused")
	svc := mocks.NewMockSessionService(t)
	svc.EXPECT().Authenticate(mock.Anything, presentedToken).Return(service.AuthenticatedSession{}, errDB)

	_, err := NewAuthenticator(svc).Authenticate(requestWithCookie(t))

	require.ErrorIs(t, err, errDB)
	require.NotErrorIs(t, err, authn.ErrInvalidCredentials)
	assert.NotErrorIs(t, err, authn.ErrNoCredentials)
}

// TestAuthenticator_Logs checks that the logs of a request name its session, behind the same middleware as in
// the application, and never hold its token.
func TestAuthenticator_Logs(t *testing.T) {
	t.Parallel()
	svc := mocks.NewMockSessionService(t)
	svc.EXPECT().Authenticate(mock.Anything, presentedToken).Return(authenticatedSession, nil)
	logger, buf := loggertest.NewJSON()
	r := router.New(router.Options{}, logger, noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	r.With(authn.Require(logger, noop.NewMeterProvider(), tracenoop.NewTracerProvider(), NewAuthenticator(svc))).
		Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	apitest.NewClient(t, r).Do(requestWithCookie(t)).AssertStatus(http.StatusNoContent)

	assert.NotContains(t, buf.String(), presentedToken, "the logs must never hold the token")
	entries := loggertest.Entries(t, buf)
	request := entries[len(entries)-1]
	require.Equal(t, "request", request["msg"])
	assert.Equal(t, sessionID.String(), request["session_id"])
	assert.Equal(t, "session", request["auth_method"])
}
