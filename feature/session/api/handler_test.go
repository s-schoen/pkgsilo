package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/authn"
	"pkgsilo/core/envelope"
	"pkgsilo/core/router"
	"pkgsilo/feature/session/service"
	"pkgsilo/feature/session/service/mocks"
	"pkgsilo/test/apitest"
	"pkgsilo/test/loggertest"
	"pkgsilo/test/telemetrytest"
)

const (
	sessionsPath = "/sessions"
	currentPath  = "/sessions/current"
	loginBody    = `{"username":"ada","password":"correct horse"}`
	newToken     = "pss_NEWTOKEN"
	// sessionCookieHeader is the Set-Cookie header of a login that creates the session of newToken.
	sessionCookieHeader = "__Host-session=pss_NEWTOKEN; Path=/; HttpOnly; Secure; SameSite=Strict"
	// expiredCookieHeader is the Set-Cookie header of a logout.
	expiredCookieHeader = "__Host-session=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Strict"
)

var (
	sessionID = uuid.MustParse("1b2c3d4e-5f60-4718-89ab-cdef01234567")
	createdAt = time.Date(2026, time.November, 17, 12, 0, 0, 123456000, time.UTC)

	adaSession = service.Session{
		ID: sessionID, IdentityID: apitest.Actor, CreatedAt: createdAt, LastSeenAt: createdAt,
		ExpiresAt: createdAt.Add(24 * time.Hour),
	}
	adaJSON = `{"id":"1b2c3d4e-5f60-4718-89ab-cdef01234567","identityId":"5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b",` +
		`"createdAt":"2026-11-17T12:00:00.123456Z","expiresAt":"2026-11-18T12:00:00.123456Z"}`
	// loginInput is what the login of loginBody sends to the service from httptest's client.
	loginInput = service.LoginInput{
		Username: "ada", Password: "correct horse", ClientIP: netip.MustParseAddr("192.0.2.1"),
	}
)

// stubAuthenticator authenticates every request as apitest.Actor with the session sessionID, under its method.
type stubAuthenticator struct {
	method string
}

func (a stubAuthenticator) Method() string  { return a.method }
func (stubAuthenticator) Challenge() string { return "" }
func (stubAuthenticator) Authenticate(*http.Request) (authn.Principal, error) {
	return authn.Principal{IdentityID: apitest.Actor, CredentialID: sessionID}, nil
}

// testHandler is the handler under test with what it records.
type testHandler struct {
	api       *apitest.Client
	svc       *mocks.MockSessionService
	telemetry *telemetrytest.Recorder
}

// newTestHandler returns a client for a handler that is backed by a mock service and mounted on the
// application router, which checks every response against Docs. Its current-session routes authenticate every
// request as apitest.Actor with the given method, session or another one.
func newTestHandler(t *testing.T, logger *slog.Logger, method string) testHandler {
	t.Helper()

	svc := mocks.NewMockSessionService(t)
	tel := telemetrytest.New(t)
	logins := authn.NewLoginRecorder(tel.MeterProvider(), tel.TracerProvider(), "password")
	authenticate := authn.Require(logger, noop.NewMeterProvider(), tracenoop.NewTracerProvider(),
		stubAuthenticator{method: method})
	r := router.New(router.Options{}, logger, noop.NewMeterProvider(), tracenoop.NewTracerProvider())
	r.Mount(sessionsPath, NewHandler(svc, logins, logger).Routes(authenticate))

	return testHandler{api: apitest.NewClient(t, r).WithDocs(sessionsPath, Docs()), svc: svc, telemetry: tel}
}

// newTestClient is newTestHandler for requests authenticated with a session.
func newTestClient(t *testing.T, logger *slog.Logger) (*apitest.Client, *mocks.MockSessionService) {
	t.Helper()

	h := newTestHandler(t, logger, "session")

	return h.api, h.svc
}

// postLogin sends a login with the body.
func postLogin(api *apitest.Client, body string) *apitest.Response {
	return api.Post(sessionsPath, body)
}

// assertLogins asserts that pkgsilo.authn.attempts counted exactly one login, with the attributes want.
func assertLogins(t *testing.T, tel *telemetrytest.Recorder, want map[string]string) {
	t.Helper()

	points := tel.SumPoints[int64](t, "pkgsilo.authn.attempts")
	require.Len(t, points, 1)
	attrs := map[string]string{}
	for _, kv := range points[0].Attributes.ToSlice() {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	assert.Equal(t, want, attrs)
}

func TestHandler_Login(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t, loggertest.Discard(), "session")
	h.svc.EXPECT().Login(mock.Anything, loginInput).
		Return(service.CreatedSession{Session: adaSession, Token: newToken}, nil)

	resp := postLogin(h.api, loginBody)

	resp.AssertCreated(currentPath, adaJSON)
	assert.Equal(t, []string{sessionCookieHeader}, resp.Header().Values("Set-Cookie"))
	assert.NotContains(t, resp.Body.String(), newToken, "only the cookie holds the token")
	assertLogins(t, h.telemetry, map[string]string{
		"pkgsilo.authn.outcome": "success", "pkgsilo.authn.method": "password",
	})
}

// TestHandler_Login_Client checks that the login passes on the session cookie that the request carries and
// what it tells about its client.
func TestHandler_Login_Client(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	want := loginInput
	want.PresentedToken, want.UserAgent = presentedToken, "Mozilla/5.0"
	svc.EXPECT().Login(mock.Anything, want).Return(service.CreatedSession{Session: adaSession, Token: newToken}, nil)

	req := httptest.NewRequest(http.MethodPost, sessionsPath, strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.AddCookie(&http.Cookie{Name: "__Host-session", Value: presentedToken})

	api.Do(req).AssertCreated(currentPath, adaJSON)
}

// TestHandler_Login_Rejected checks that every rejected login gets the same 401 without a cookie, is counted
// with its reason and logged with it and its identity, if one has the username, but never with the username
// or the password.
func TestHandler_Login_Rejected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		rejected *service.RejectedError
		reason   string
	}{
		{name: "unknown", rejected: &service.RejectedError{Reason: service.ErrUnknownCredentials}, reason: "unknown"},
		{
			name:     "no password",
			rejected: &service.RejectedError{IdentityID: apitest.Actor, Reason: service.ErrUnknownCredentials},
			reason:   "unknown",
		},
		{
			name:     "invalid",
			rejected: &service.RejectedError{IdentityID: apitest.Actor, Reason: service.ErrInvalidPassword},
			reason:   "invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			logger, buf := loggertest.NewJSON()
			h := newTestHandler(t, logger, "session")
			h.svc.EXPECT().Login(mock.Anything, loginInput).
				Return(service.CreatedSession{}, fmt.Errorf("log in: %w", tt.rejected))

			resp := postLogin(h.api, loginBody)

			resp.AssertProblem(envelope.UnauthorizedProblem.New(envelope.Occurrence{}))
			assert.Empty(t, resp.Header().Values("Set-Cookie"))
			assert.Empty(t, resp.Header().Values("WWW-Authenticate"))
			assertLogins(t, h.telemetry, map[string]string{
				"pkgsilo.authn.outcome": "rejected", "pkgsilo.authn.method": "password", "pkgsilo.authn.reason": tt.reason,
			})
			for _, secret := range []string{`"ada"`, "horse"} {
				assert.NotContains(t, buf.String(), secret)
			}
			entries := loggertest.Entries(t, buf)
			require.Len(t, entries, 2, "the rejection and the request")
			assert.Equal(t, "INFO", entries[0]["level"])
			assert.Equal(t, "login rejected", entries[0]["msg"])
			assert.Equal(t, tt.reason, entries[0]["reason"])
			if tt.rejected.IdentityID == uuid.Nil() {
				assert.NotContains(t, entries[0], "identity_id")
			} else {
				assert.Equal(t, apitest.Actor.String(), entries[0]["identity_id"])
			}
		})
	}
}

func TestHandler_Login_InvalidBody(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		body string
		want []envelope.ValidationError
	}{
		"missing": {body: `{}`, want: []envelope.ValidationError{
			{Pointer: "#/password", Detail: "cannot be blank"}, {Pointer: "#/username", Detail: "cannot be blank"},
		}},
		"empty password": {body: `{"username":"ada","password":""}`, want: []envelope.ValidationError{
			{Pointer: "#/password", Detail: "cannot be blank"},
		}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test on any call.
			api, _ := newTestClient(t, loggertest.Discard())

			postLogin(api, tt.body).AssertInvalidBody(tt.want...)
		})
	}
}

// TestHandler_Login_UnexpectedError checks that a failed login is logged and answered with a 500 without
// details, and counted as an error.
func TestHandler_Login_UnexpectedError(t *testing.T) {
	t.Parallel()
	logger, buf := loggertest.NewJSON()
	h := newTestHandler(t, logger, "session")
	h.svc.EXPECT().Login(mock.Anything, loginInput).Return(service.CreatedSession{}, errors.New("connection refused"))

	resp := postLogin(h.api, loginBody)

	resp.AssertInternalError()
	assert.Empty(t, resp.Header().Values("Set-Cookie"))
	entries := loggertest.Entries(t, buf)
	require.NotEmpty(t, entries)
	assert.Equal(t, "session request failed", entries[0]["msg"])
	assert.Equal(t, "connection refused", entries[0]["error"])
	assert.NotContains(t, buf.String(), "horse")
	assertLogins(t, h.telemetry, map[string]string{"pkgsilo.authn.outcome": "error", "pkgsilo.authn.method": "password"})
}

func TestHandler_GetCurrent(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Get(mock.Anything, apitest.Actor, sessionID).Return(adaSession, nil)

	api.Get(currentPath).AssertResource(adaJSON)
}

func TestHandler_DeleteCurrent(t *testing.T) {
	t.Parallel()
	api, svc := newTestClient(t, loggertest.Discard())
	svc.EXPECT().Delete(mock.Anything, apitest.Actor, sessionID).Return(nil)

	resp := api.Delete(currentPath)

	resp.AssertNoContent()
	assert.Equal(t, []string{expiredCookieHeader}, resp.Header().Values("Set-Cookie"))
}

// TestHandler_Current_OtherMethod checks that a request authenticated otherwise than with a session, such as
// with an API key, has no current session.
func TestHandler_Current_OtherMethod(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			// The mock fails the test on any call.
			h := newTestHandler(t, loggertest.Discard(), "api_key")

			resp := h.api.Send(method, currentPath, "")

			resp.AssertProblem(sessionNotFoundProblem)
			assert.Empty(t, resp.Header().Values("Set-Cookie"))
		})
	}
}

func TestHandler_Current_ServiceErrors(t *testing.T) {
	t.Parallel()
	t.Run("get", func(t *testing.T) {
		t.Parallel()
		api, svc := newTestClient(t, loggertest.Discard())
		svc.EXPECT().Get(mock.Anything, apitest.Actor, sessionID).
			Return(service.Session{}, fmt.Errorf("get: %w", service.ErrSessionNotFound))

		api.Get(currentPath).AssertProblem(sessionNotFoundProblem)
	})
	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		api, svc := newTestClient(t, loggertest.Discard())
		svc.EXPECT().Delete(mock.Anything, apitest.Actor, sessionID).Return(errors.New("connection refused"))

		resp := api.Delete(currentPath)

		resp.AssertInternalError()
		assert.Empty(t, resp.Header().Values("Set-Cookie"))
	})
}
