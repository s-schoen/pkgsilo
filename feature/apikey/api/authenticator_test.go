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

	"pkgsilo/core/authn"
	"pkgsilo/core/router"
	"pkgsilo/feature/apikey/service"
	"pkgsilo/feature/apikey/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
)

const presentedKey = "psk_PRESENTEDKEY"

// authenticatedKey is ciKey as Authenticate returns it.
var authenticatedKey = service.AuthenticatedKey{ID: ciKey.ID, IdentityID: ciIdentityID}

func requestWithKey(t *testing.T, key string) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", key)

	return req
}

func TestAuthenticator_Method(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "api_key", NewAuthenticator(mocks.NewMockAPIKeyService(t)).Method())
}

func TestAuthenticator_Challenge(t *testing.T) {
	t.Parallel()

	assert.Equal(t, `ApiKey realm="pkgsilo"`, NewAuthenticator(mocks.NewMockAPIKeyService(t)).Challenge())
}

func TestAuthenticator_Authenticate(t *testing.T) {
	t.Parallel()
	svc := mocks.NewMockAPIKeyService(t)
	svc.EXPECT().Authenticate(mock.Anything, presentedKey).Return(authenticatedKey, nil)

	got, err := NewAuthenticator(svc).Authenticate(requestWithKey(t, presentedKey))
	require.NoError(t, err)

	assert.Equal(t, authn.Principal{IdentityID: ciIdentityID}, got)
}

func TestAuthenticator_Authenticate_NoCredentials(t *testing.T) {
	t.Parallel()
	// The mock fails the test on any call.
	authenticator := NewAuthenticator(mocks.NewMockAPIKeyService(t))

	t.Run("no header", func(t *testing.T) {
		t.Parallel()
		_, err := authenticator.Authenticate(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

		assert.ErrorIs(t, err, authn.ErrNoCredentials)
	})
	t.Run("empty header", func(t *testing.T) {
		t.Parallel()
		_, err := authenticator.Authenticate(requestWithKey(t, ""))

		assert.ErrorIs(t, err, authn.ErrNoCredentials)
	})
}

func TestAuthenticator_Authenticate_Rejected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cause  error
		reason error
	}{
		{service.ErrMalformedKey, authn.ErrMalformedCredentials},
		{service.ErrUnknownKey, authn.ErrUnknownCredentials},
		{service.ErrExpiredKey, authn.ErrExpiredCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.cause.Error(), func(t *testing.T) {
			t.Parallel()
			svc := mocks.NewMockAPIKeyService(t)
			svc.EXPECT().Authenticate(mock.Anything, presentedKey).
				Return(service.AuthenticatedKey{}, fmt.Errorf("authenticate: %w", tt.cause))

			_, err := NewAuthenticator(svc).Authenticate(requestWithKey(t, presentedKey))

			require.ErrorIs(t, err, tt.reason)
			require.ErrorIs(t, err, authn.ErrInvalidCredentials)
			assert.ErrorContains(t, err, tt.cause.Error(), "the logs must tell the cause")
		})
	}
}

func TestAuthenticator_Authenticate_UnexpectedError(t *testing.T) {
	t.Parallel()
	errDB := errors.New("connection refused")
	svc := mocks.NewMockAPIKeyService(t)
	svc.EXPECT().Authenticate(mock.Anything, presentedKey).Return(service.AuthenticatedKey{}, errDB)

	_, err := NewAuthenticator(svc).Authenticate(requestWithKey(t, presentedKey))

	require.ErrorIs(t, err, errDB)
	require.NotErrorIs(t, err, authn.ErrInvalidCredentials)
	assert.NotErrorIs(t, err, authn.ErrNoCredentials)
}

// TestAuthenticator_Logs checks what the logs of a request say about its API key, behind the same
// middleware as in the application.
func TestAuthenticator_Logs(t *testing.T) {
	t.Parallel()
	errExpired := fmt.Errorf("api key %s: %w", ciKey.ID, service.ErrExpiredKey)
	tests := []struct {
		name       string
		err        error
		wantStatus int
		// wantKeyID is whether the request's log record names the API key.
		wantKeyID bool
	}{
		{name: "accepted", wantStatus: http.StatusNoContent, wantKeyID: true},
		{name: "rejected", err: errExpired, wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := mocks.NewMockAPIKeyService(t)
			svc.EXPECT().Authenticate(mock.Anything, presentedKey).Return(authenticatedKey, tt.err)
			logger, buf := loggertest.NewJSON()
			r := router.New(router.Options{}, logger)
			r.With(authn.Require(logger, noop.NewMeterProvider(), NewAuthenticator(svc))).
				Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

			apitest.NewClient(t, r).Do(requestWithKey(t, presentedKey)).AssertStatus(tt.wantStatus)

			assert.NotContains(t, buf.String(), presentedKey, "the logs must never hold the key")
			entries := loggertest.Entries(t, buf)
			request := entries[len(entries)-1]
			require.Equal(t, "request", request["msg"])
			if tt.wantKeyID {
				assert.Equal(t, ciKey.ID.String(), request["api_key_id"])
			} else {
				assert.NotContains(t, request, "api_key_id")
				assert.Equal(t, "expired", entries[0]["reason"])
				assert.Contains(t, entries[0]["error"], ciKey.ID.String(), "the rejection must name the expired key")
			}
		})
	}
}
