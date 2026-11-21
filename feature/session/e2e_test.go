// End-to-end tests of logging in with a password and of the session cookie: each test drives the whole app
// through /api/v1 and /mcp, as cmd/main.go serves it, on a fresh database. The doc comment of each test lists
// its steps in order, each with the result it expects; the numbered comments in the test mark where each step
// starts.
package session_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/envelope"
	identityapi "pkgsilo/feature/identity/api"
	sessionapi "pkgsilo/feature/session/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/apptest"
)

// The problems of the session endpoints and the middleware in front of them, written out independently of the
// code: clients tell problems apart by them, so a changed slug, title or status must fail these tests.
var (
	sessionNotFound    = envelope.DefineProblemType("session-not-found", "Session not found", http.StatusNotFound)
	crossOriginRequest = envelope.DefineProblemType("cross-origin-request", "Cross-origin request",
		http.StatusForbidden)
	unauthorized = envelope.UnauthorizedProblem.New(envelope.Occurrence{})
)

const (
	sessionsPath = "/api/v1/sessions"
	currentPath  = "/api/v1/sessions/current"
	adaPassword  = "correct horse"
	// expiredCookie is the Set-Cookie header of a logout.
	expiredCookie = "__Host-session=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Strict"
)

type (
	session  = sessionapi.SessionResponse
	identity = identityapi.IdentityResponse
)

// newAda creates the identity ada with the password adaPassword through the client admin and returns it.
func newAda(admin *apitest.Client) identity {
	ada, _ := apptest.NewIdentity(admin, "ada")
	apptest.SetPassword(admin, ada.ID, adaPassword)

	return ada
}

// assertUnauthorized asserts the 401 of a request with a rejected or no session cookie, which challenges only
// for API keys, since cookies have no scheme.
func assertUnauthorized(t *testing.T, resp *apitest.Response) {
	t.Helper()

	resp.AssertProblem(unauthorized)
	assert.Equal(t, []string{`ApiKey realm="pkgsilo"`}, resp.Header().Values("WWW-Authenticate"))
}

// TestSessions_LogInAndOut takes ada through a login, a request, her current session and a logout:
//
//  1. Log in as ada: 201 with the session and the cookie __Host-session, Secure, HttpOnly, SameSite=Strict,
//     which expires with the session, so that it outlasts a restart of the browser.
//  2. With the cookie alone, get ada: 200, so the session authenticates as her.
//  3. Get the current session: the session of the login.
//  4. Log out: 204, and the cookie expires.
//  5. Get the current session with the old cookie: 401.
func TestSessions_LogInAndOut(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	ada := newAda(api)

	// 1. Log in.
	resp := api.Anonymous().Post(sessionsPath, `{"username":"ada","password":"correct horse"}`)
	created := resp.DecodeResource[session](http.StatusCreated)
	assert.Equal(t, currentPath, resp.Header().Get("Location"))
	assert.Equal(t, ada.ID, created.IdentityID.Value)
	assert.WithinDuration(t, time.Now(), created.CreatedAt.Value, time.Minute)
	assert.Equal(t, created.CreatedAt.Value.Add(24*time.Hour), created.ExpiresAt.Value)
	setCookies := resp.Header().Values("Set-Cookie")
	require.Len(t, setCookies, 1)
	assert.Regexp(t, `^__Host-session=pss_[A-Z2-7]{52}; Path=/; Max-Age=86400; HttpOnly; Secure; SameSite=Strict$`,
		setCookies[0], "the cookie must last as long as the session at most, counted by the browser's clock")
	asAda := api.Anonymous().WithHeader("Cookie", apptest.SessionCookie(t, resp))

	// 2. Get ada.
	assert.Equal(t, ada.ID, asAda.Get("/api/v1/identities/" + ada.ID.String()).DecodeResource[identity](http.StatusOK).ID)

	// 3. Get the current session.
	assert.Equal(t, created, asAda.Get(currentPath).DecodeResource[session](http.StatusOK))

	// 4. Log out.
	logout := asAda.Delete(currentPath)
	logout.AssertNoContent()
	assert.Equal(t, []string{expiredCookie}, logout.Header().Values("Set-Cookie"))

	// 5. Use the old cookie.
	assertUnauthorized(t, asAda.Get(currentPath))
}

// TestSessions_RejectedLogins logs in with credentials that aren't accepted, which all get the same 401
// without a cookie, whatever the reason:
//
//  1. Log in with a username that no identity has.
//  2. Log in as ada with a wrong password.
//  3. Log in as grace, who has no password.
//  4. Compare the responses: they're identical but for their request IDs, and none sets a cookie or a
//     challenge.
func TestSessions_RejectedLogins(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	newAda(api)
	apptest.NewIdentity(api, "grace")

	// 1.–3. Log in.
	var responses []*apitest.Response
	for _, body := range []string{
		`{"username":"nobody","password":"correct horse"}`,
		`{"username":"ada","password":"correct horse "}`,
		`{"username":"grace","password":"correct horse"}`,
	} {
		responses = append(responses, api.Anonymous().Post(sessionsPath, body))
	}

	// 4. Compare, but for the request IDs.
	for _, resp := range responses {
		resp.AssertProblem(unauthorized)
		resp.Header().Del("X-Request-ID")
		assert.Equal(t, responses[0].Header(), resp.Header())
		assert.Equal(t, responses[0].Body.String(), resp.Body.String())
		assert.Empty(t, resp.Header().Values("Set-Cookie"))
		assert.Empty(t, resp.Header().Values("WWW-Authenticate"))
	}
}

// TestSessions_LoginEndsPresentedSession logs in with a session cookie, as after a cookie was planted in the
// browser:
//
//  1. Log in as ada: the first session.
//  2. Log in again, carrying the first session's cookie: a second session.
//  3. Use the first cookie: 401, since the second login ended its session.
//  4. Use the second cookie: 200.
func TestSessions_LoginEndsPresentedSession(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	newAda(api)

	// 1. Log in.
	_, first := apptest.LogIn(t, api, "ada", adaPassword)

	// 2. Log in with the first cookie.
	resp := first.Post(sessionsPath, `{"username":"ada","password":"correct horse"}`)
	resp.AssertStatus(http.StatusCreated)
	second := api.Anonymous().WithHeader("Cookie", apptest.SessionCookie(t, resp))

	// 3. Use the first cookie.
	assertUnauthorized(t, first.Get(currentPath))

	// 4. Use the second cookie.
	second.Get(currentPath).AssertStatus(http.StatusOK)
}

// TestSessions_CrossSiteRequests sends requests that a browser marks as cross-site, which a cookie must not
// authenticate:
//
//  1. Log in as ada.
//  2. With her cookie, create an identity cross-site: 403 cross-origin-request, and nothing is created.
//  3. Log out cross-site: 403 cross-origin-request, and the session still works.
//  4. Log in cross-site: 403 cross-origin-request, without a cookie.
func TestSessions_CrossSiteRequests(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	newAda(api)

	// 1. Log in.
	_, asAda := apptest.LogIn(t, api, "ada", adaPassword)
	crossSite := asAda.WithHeader("Sec-Fetch-Site", "cross-site")

	// 2. Create an identity.
	crossSite.Post("/api/v1/identities", `{"username":"mallory","displayName":"Mallory"}`).
		AssertProblemType(crossOriginRequest)
	assert.Len(t, api.Get("/api/v1/identities").DecodeCollection[identity](http.StatusOK).Data, 2)

	// 3. Log out.
	crossSite.Delete(currentPath).AssertProblemType(crossOriginRequest)
	asAda.Get(currentPath).AssertStatus(http.StatusOK)

	// 4. Log in.
	resp := api.Anonymous().WithHeader("Sec-Fetch-Site", "cross-site").
		Post(sessionsPath, `{"username":"ada","password":"correct horse"}`)
	resp.AssertProblemType(crossOriginRequest)
	assert.Empty(t, resp.Header().Values("Set-Cookie"))
}

// TestSessions_Expiry ages sessions in the database past the default lifetimes, so that no step depends on how
// long the steps before it took:
//
//  1. Idle timeout: log in, and use the session at once: 200. Set its last use back by the idle timeout and use
//     it: 401.
//  2. Maximum age: log in, and use the session at once: 200. Set its expiry to now and use it: 401.
func TestSessions_Expiry(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// age is the statement that sets a time of the session with the ID $1 to $2.
		age string
		to  time.Time
	}{
		{
			name: "idle timeout",
			age:  `UPDATE sessions SET last_seen_at = $2 WHERE id = $1`,
			to:   time.Now().Add(-apptest.DefaultSessionLifetimes.IdleTimeout),
		},
		{name: "maximum age", age: `UPDATE sessions SET expires_at = $2 WHERE id = $1`, to: time.Now()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api, db := apptest.NewWithDB(t)
			newAda(api)

			// 1., 2. Log in, use, age and use again.
			created, asAda := apptest.LogIn(t, api, "ada", adaPassword)
			asAda.Get(currentPath).AssertStatus(http.StatusOK)
			_, err := db.ExecContext(t.Context(), tt.age, created.ID, tt.to)
			require.NoError(t, err)
			assertUnauthorized(t, asAda.Get(currentPath))
		})
	}
}

// TestSessions_APIKeyWins sends requests that carry both an API key and a session cookie:
//
//  1. Log in as ada.
//  2. With admin's API key and ada's cookie, get the current session: 404 session-not-found, since the API key
//     authenticates the request.
//  3. With the same, log out: 404 session-not-found, and ada's session still works.
func TestSessions_APIKeyWins(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	newAda(api)

	// 1. Log in.
	_, asAda := apptest.LogIn(t, api, "ada", adaPassword)
	both := api.WithHeader("Cookie", asAda.Header().Get("Cookie"))

	// 2. Get the current session.
	both.Get(currentPath).AssertProblemType(sessionNotFound)

	// 3. Log out.
	both.Delete(currentPath).AssertProblemType(sessionNotFound)
	asAda.Get(currentPath).AssertStatus(http.StatusOK)
}

// TestSessions_NotOnMCP sends ada's session cookie to the MCP endpoint, which takes API keys only:
//
//  1. Log in as ada.
//  2. Ping /mcp with her cookie alone: 401.
func TestSessions_NotOnMCP(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	newAda(api)

	// 1. Log in.
	_, asAda := apptest.LogIn(t, api, "ada", adaPassword)

	// 2. Ping /mcp.
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	assertUnauthorized(t, asAda.Do(req))
}
