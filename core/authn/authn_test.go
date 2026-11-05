package authn_test

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/authn"
	"pkgsilo/core/authn/mocks"
	"pkgsilo/core/envelope"
	"pkgsilo/core/middleware"
	"pkgsilo/test/loggertest"
)

const unauthorizedBody = `{"type":"about:blank","title":"Unauthorized","status":401}`

// newAuthenticator returns a mock of the given method, whose challenge is the method's name. Calling
// its Authenticate fails the test unless the test expects it.
func newAuthenticator(t *testing.T, method string) *mocks.MockAuthenticator {
	t.Helper()

	authenticator := mocks.NewMockAuthenticator(t)
	authenticator.EXPECT().Challenge().Return(method)
	authenticator.EXPECT().Method().Return(method).Maybe()

	return authenticator
}

// serve sends a request through Require to a handler, and returns the response and the principal
// that the handler got, which is nil if it didn't run.
func serve(
	t *testing.T, logger *slog.Logger, authenticators ...authn.Authenticator,
) (*httptest.ResponseRecorder, *authn.Principal) {
	t.Helper()

	var got *authn.Principal
	h := authn.Require(logger, authenticators...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal := authn.PrincipalFrom(r.Context())
		got = &principal
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	return rec, got
}

func TestRequire_FirstMethodThatRecognizesTheCredentialDecides(t *testing.T) {
	t.Parallel()
	id := uuid.NewV7()
	first := newAuthenticator(t, "first")
	first.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{}, authn.ErrNoCredentials)
	second := newAuthenticator(t, "second")
	second.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{IdentityID: id}, nil)
	third := newAuthenticator(t, "third")

	rec, got := serve(t, loggertest.Discard(), first, second, third)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	require.NotNil(t, got)
	assert.Equal(t, authn.Principal{IdentityID: id, Method: "second"}, *got)
}

func TestRequire_AnswersMissingCredentialsWith401(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	first := newAuthenticator(t, "first")
	first.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{}, authn.ErrNoCredentials)
	second := newAuthenticator(t, "second")
	second.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{}, authn.ErrNoCredentials)

	rec, got := serve(t, logger, first, second)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, envelope.ProblemJSONMediaType, rec.Header().Get("Content-Type"))
	assert.JSONEq(t, unauthorizedBody, rec.Body.String())
	assert.Equal(t, []string{"first", "second"}, rec.Header().Values("WWW-Authenticate"))
	assert.Nil(t, got)
	assert.Empty(t, buf.String())
}

func TestRequire_AnswersRejectedCredentialsWith401(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	first := newAuthenticator(t, "first")
	first.EXPECT().Authenticate(mock.Anything).
		Return(authn.Principal{}, fmt.Errorf("%w: expired", authn.ErrInvalidCredentials))
	second := newAuthenticator(t, "second")

	rec, got := serve(t, logger, first, second)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, unauthorizedBody, rec.Body.String(), "the response doesn't say why")
	assert.Equal(t, []string{"first", "second"}, rec.Header().Values("WWW-Authenticate"))
	assert.Nil(t, got)

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "INFO", entries[0]["level"])
	assert.Equal(t, "authentication rejected", entries[0]["msg"])
	assert.Equal(t, "first", entries[0]["auth_method"])
	assert.Equal(t, "invalid credentials: expired", entries[0]["reason"])
}

func TestRequire_AnswersOtherErrorsWith500(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	first := newAuthenticator(t, "first")
	first.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{}, errors.New("database down"))
	second := newAuthenticator(t, "second")

	rec, got := serve(t, logger, first, second)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"type":"about:blank","title":"Internal Server Error","status":500}`, rec.Body.String())
	assert.Empty(t, rec.Header().Values("WWW-Authenticate"))
	assert.Nil(t, got)

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 1)
	assert.Equal(t, "ERROR", entries[0]["level"])
	assert.Equal(t, "authentication failed", entries[0]["msg"])
	assert.Equal(t, "first", entries[0]["auth_method"])
	assert.Equal(t, "database down", entries[0]["error"])
}

func TestRequire_AddsThePrincipalToTheLogsOfTheRequest(t *testing.T) {
	t.Parallel()
	id := uuid.NewV7()
	logger, buf := loggertest.NewJSON()
	authenticator := newAuthenticator(t, "first")
	authenticator.EXPECT().Authenticate(mock.Anything).Return(authn.Principal{IdentityID: id}, nil)

	h := middleware.RequestLogger(logger)(authn.Require(logger, authenticator)(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			logger.InfoContext(r.Context(), "handled")
		}),
	))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	entries := loggertest.Entries(t, buf)
	require.Len(t, entries, 2)
	for _, entry := range entries {
		assert.Equal(t, id.String(), entry["identity_id"], entry["msg"])
		assert.Equal(t, "first", entry["auth_method"], entry["msg"])
	}
}

func TestRequire_PanicsWithoutAuthenticators(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, "authn: Require needs at least one authenticator", func() {
		authn.Require(loggertest.Discard())
	})
}

func TestPrincipalFrom_PanicsWithoutPrincipal(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, "authn: no principal in the context: the route is not behind Require", func() {
		authn.PrincipalFrom(t.Context())
	})
}
