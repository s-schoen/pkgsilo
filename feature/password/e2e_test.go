// End-to-end tests of the password endpoints and tools: each test drives the whole app through /api/v1 and
// /mcp, as cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps in
// order, each with the result it expects; the numbered comments in the test mark where each step starts.
package password_test

import (
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/envelope"
	"pkgsilo/core/mcpserver"
	identityapi "pkgsilo/feature/identity/api"
	passwordapi "pkgsilo/feature/password/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/apptest"
)

// The problem types of the password endpoints, written out independently of the code: clients tell problems
// apart by them, so a changed slug, title or status must fail these tests.
var (
	passwordNotFound = envelope.DefineProblemType("password-not-found", "Password not found", http.StatusNotFound)
	identityNotFound = envelope.DefineProblemType("identity-not-found", "Identity not found", http.StatusNotFound)
	invalidBody      = envelope.DefineProblemType("invalid-body", "Invalid request body", http.StatusUnprocessableEntity)
	forbidden        = envelope.ProblemType{Type: "about:blank", Title: "Forbidden", Status: http.StatusForbidden}
	unauthorized     = envelope.UnauthorizedProblem.New(envelope.Occurrence{})
)

// The tool errors of the password tools, written out independently of the code: agents tell errors apart by
// their codes and act on their messages, so a changed code or message must fail these tests.
var errPasswordNotFound = mcpserver.DefineError("password_not_found",
	"The identity has no password, or the policies do not allow you to read it.")

type password = passwordapi.PasswordResponse

// passwordPathOf returns the path of the password of the identity with the given ID.
func passwordPathOf(id uuid.UUID) string {
	return "/api/v1/identities/" + id.String() + "/password"
}

// currentSessionPath is the path of the session that a request is authenticated with.
const (
	currentSessionPath = "/api/v1/sessions/current"
	// expiredCookie is the Set-Cookie header that deletes the session cookie.
	expiredCookie = "__Host-session=; Path=/; Max-Age=0; HttpOnly; Secure; SameSite=Strict"
)

// assertSessionsEnded asserts that every client, each sending only a session cookie, is rejected.
func assertSessionsEnded(sessions ...*apitest.Client) {
	for _, session := range sessions {
		session.Get(currentSessionPath).AssertProblem(unauthorized)
	}
}

// assertAPIKeyWorks asserts that the client, which authenticates as the identity with the given ID with an
// API key, still gets the identity.
func assertAPIKeyWorks(t *testing.T, client *apitest.Client, identityID uuid.UUID) {
	t.Helper()

	got := client.Get("/api/v1/identities/" + identityID.String()).
		DecodeResource[identityapi.IdentityResponse](http.StatusOK)
	assert.Equal(t, identityID, got.ID)
}

// assertCurrentPasswordProblem asserts an invalid-body problem on currentPassword with the given detail.
func assertCurrentPasswordProblem(t *testing.T, resp *apitest.Response, detail string) {
	t.Helper()

	resp.AssertProblem(invalidBody.New(envelope.Occurrence{
		Detail: "The request body contains invalid members.",
		Extensions: map[string]any{envelope.ValidationErrorsExtension: []envelope.ValidationError{
			{Pointer: "#/currentPassword", Detail: detail},
		}},
	}))
}

// TestPasswords_Lifecycle takes ada's password through a reset by admin, a change by ada and a delete by
// admin:
//
//  1. Create ada, then get her password: 404 password-not-found.
//  2. As admin, reset her password without a current one: 200 with when it was set.
//  3. As ada, get her password: the same.
//  4. As ada, change it without the current password, and with a wrong one: 422 invalid-body on
//     currentPassword for both, and the password unchanged.
//  5. As ada, change it with the current password: 200, set later than before.
//  6. As ada, delete it: 403 Forbidden, since the seeded policy only lets her change it.
//  7. As admin, delete it: 204. Get it as admin and as ada: 404 password-not-found.
//  8. As ada, change it again: 404 password-not-found, since only a reset sets a first password.
func TestPasswords_Lifecycle(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	path := passwordPathOf(ada.ID)
	api.Get(path).AssertProblemType(passwordNotFound)

	// 2. Reset.
	reset := api.Put(path, `{"newPassword":"correct horse"}`).DecodeResource[password](http.StatusOK)
	assert.WithinDuration(t, time.Now(), reset.SetAt.Value, time.Minute)

	// 3. Get as ada.
	assert.Equal(t, reset, asAda.Get(path).DecodeResource[password](http.StatusOK))

	// 4. Change without the right current password.
	assertCurrentPasswordProblem(t, asAda.Put(path, `{"newPassword":"battery staple"}`),
		"is required to change the password")
	assertCurrentPasswordProblem(t,
		asAda.Put(path, `{"newPassword":"battery staple","currentPassword":"correct horse "}`),
		"does not match the current password")
	assert.Equal(t, reset, api.Get(path).DecodeResource[password](http.StatusOK))

	// 5. Change.
	changed := asAda.Put(path, `{"newPassword":"battery staple","currentPassword":"correct horse"}`).
		DecodeResource[password](http.StatusOK)
	assert.True(t, changed.SetAt.Value.After(reset.SetAt.Value), "a change must set a new time")
	assertCurrentPasswordProblem(t,
		asAda.Put(path, `{"newPassword":"staple","currentPassword":"correct horse"}`),
		"does not match the current password")

	// 6. Delete as ada.
	asAda.Delete(path).AssertProblemType(forbidden)

	// 7. Delete as admin.
	api.Delete(path).AssertNoContent()
	api.Get(path).AssertProblemType(passwordNotFound)
	asAda.Get(path).AssertProblemType(passwordNotFound)

	// 8. Change again.
	asAda.Put(path, `{"newPassword":"correct horse","currentPassword":"battery staple"}`).
		AssertProblemType(passwordNotFound)
}

// TestPasswords_ChangeEndsSessions checks that changing a password ends every session of its identity, the
// one that changed it included, so that a stolen session doesn't survive:
//
//  1. Create ada, reset her password as admin, and log in as her twice: two sessions.
//  2. With the first session, change her password: 200, and the response deletes the session cookie.
//  3. Use both sessions: 401 for both.
//  4. With her API key, get ada: 200, since API keys are unaffected.
//  5. Log in with the new password: 201.
func TestPasswords_ChangeEndsSessions(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create and log in twice.
	ada, asAda := apptest.NewIdentity(api, "ada")
	apptest.SetPassword(api, ada.ID, "correct horse")
	_, first := apptest.LogIn(t, api, "ada", "correct horse")
	_, second := apptest.LogIn(t, api, "ada", "correct horse")

	// 2. Change.
	resp := first.Put(passwordPathOf(ada.ID), `{"newPassword":"battery staple","currentPassword":"correct horse"}`)
	resp.AssertStatus(http.StatusOK)
	assert.Equal(t, []string{expiredCookie}, resp.Header().Values("Set-Cookie"))

	// 3. Use the sessions.
	assertSessionsEnded(first, second)

	// 4. Use the API key.
	assertAPIKeyWorks(t, asAda, ada.ID)

	// 5. Log in again.
	apptest.LogIn(t, api, "ada", "battery staple")
}

// TestPasswords_ResetAndDeleteEndSessions checks that an admin who resets or deletes a password ends every
// session of its identity:
//
//  1. Create ada, reset her password as admin, and log in as her twice: two sessions.
//  2. As admin, reset her password: 200, without deleting a cookie, since admin's API key isn't ada's session.
//  3. Use both sessions: 401 for both. With her API key, get ada: 200.
//  4. Log in as ada with the new password, twice: two new sessions.
//  5. As admin, delete her password: 204, without deleting a cookie.
//  6. Use both new sessions: 401 for both. With her API key, get ada: 200.
func TestPasswords_ResetAndDeleteEndSessions(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create and log in twice.
	ada, asAda := apptest.NewIdentity(api, "ada")
	path := passwordPathOf(ada.ID)
	apptest.SetPassword(api, ada.ID, "correct horse")
	_, first := apptest.LogIn(t, api, "ada", "correct horse")
	_, second := apptest.LogIn(t, api, "ada", "correct horse")

	// 2. Reset.
	resp := api.Put(path, `{"newPassword":"battery staple"}`)
	resp.AssertStatus(http.StatusOK)
	assert.Empty(t, resp.Header().Values("Set-Cookie"))

	// 3. Use the sessions and the API key.
	assertSessionsEnded(first, second)
	assertAPIKeyWorks(t, asAda, ada.ID)

	// 4. Log in twice again.
	_, third := apptest.LogIn(t, api, "ada", "battery staple")
	_, fourth := apptest.LogIn(t, api, "ada", "battery staple")

	// 5. Delete.
	resp = api.Delete(path)
	resp.AssertNoContent()
	assert.Empty(t, resp.Header().Values("Set-Cookie"))

	// 6. Use the sessions and the API key.
	assertSessionsEnded(third, fourth)
	assertAPIKeyWorks(t, asAda, ada.ID)
}

// TestPasswords_NoFirstPasswordForYourself checks that an identity can't give itself a first password, so that
// a leaked API key can't create a password that would outlive the key's revocation:
//
//  1. Create ada, then as ada set her password: 404 password-not-found.
//  2. As admin, get her password: still none.
func TestPasswords_NoFirstPasswordForYourself(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Set as ada.
	ada, asAda := apptest.NewIdentity(api, "ada")
	path := passwordPathOf(ada.ID)
	asAda.Put(path, `{"newPassword":"correct horse"}`).AssertProblemType(passwordNotFound)

	// 2. Nothing written.
	api.Get(path).AssertProblemType(passwordNotFound)
}

// TestPasswords_ResetOwn checks that admin, whom the administrators policy allows everything, resets its own
// password without knowing it, as the operator does on the first start with the bootstrap API key:
//
//  1. As admin, set admin's password twice, without a current password: 200 for both.
func TestPasswords_ResetOwn(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	admin := api.Get("/api/v1/identities").DecodeCollection[identityapi.IdentityResponse](http.StatusOK).Data[0]
	path := passwordPathOf(admin.ID)

	// 1. Reset twice.
	api.Put(path, `{"newPassword":"correct horse"}`).AssertStatus(http.StatusOK)
	api.Put(path, `{"newPassword":"battery staple"}`).AssertStatus(http.StatusOK)
}

// TestPasswords_OfHiddenIdentity checks that ada, by the seeded policies, can't use the password of grace,
// whom she can't read, and can't tell grace from an identity that doesn't exist:
//
//  1. Create ada and grace, and as admin reset grace's password: 200.
//  2. As ada, get, set and delete grace's password: the same 404 identity-not-found, to the byte, as for an
//     identity that doesn't exist.
//  3. As admin, get grace's password: unchanged.
func TestPasswords_OfHiddenIdentity(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	_, asAda := apptest.NewIdentity(api, "ada")
	grace, _ := apptest.NewIdentity(api, "grace")
	gracePath := passwordPathOf(grace.ID)
	set := api.Put(gracePath, `{"newPassword":"correct horse"}`).DecodeResource[password](http.StatusOK)
	missingPath := passwordPathOf(uuid.NewV4())

	// 2. Use grace's password.
	assertLikeMissing(t, asAda.Get, gracePath, missingPath)
	assertLikeMissing(t, func(target string) *apitest.Response {
		return asAda.Put(target, `{"newPassword":"mine","currentPassword":"correct horse"}`)
	}, gracePath, missingPath)
	assertLikeMissing(t, asAda.Delete, gracePath, missingPath)

	// 3. Nothing written.
	assert.Equal(t, set, api.Get(gracePath).DecodeResource[password](http.StatusOK))
}

// assertLikeMissing asserts that send answers target with the same identity-not-found problem, to the byte,
// as missingTarget, which refers to an identity that doesn't exist.
func assertLikeMissing(t *testing.T, send func(target string) *apitest.Response, target, missingTarget string) {
	t.Helper()

	hidden := send(target)
	hidden.AssertProblemType(identityNotFound)
	assert.Equal(t, send(missingTarget).Body.String(), hidden.Body.String(), "%s must look like it's missing", target)
}

// TestPasswords_MCP checks the password tools, which never take a password:
//
//  1. Create ada, reset her password through the REST API: 200, and log in as her.
//  2. Get it with password_get: when it was set, as the REST API returns it.
//  3. Delete it with password_delete: empty output.
//  4. Get it again with password_get: password_not_found, and the REST API agrees. Her session: 401.
func TestPasswords_MCP(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	mcp := apptest.MCP(t, api)

	// 1. Reset.
	ada, _ := apptest.NewIdentity(api, "ada")
	path := passwordPathOf(ada.ID)
	set := api.Put(path, `{"newPassword":"correct horse"}`).DecodeResource[password](http.StatusOK)
	_, session := apptest.LogIn(t, api, "ada", "correct horse")
	args := `{"identityId":"` + ada.ID.String() + `"}`

	// 2. Get.
	assert.Equal(t, set, mcp.Call("password_get", args).Decode[password]())

	// 3. Delete.
	mcp.Call("password_delete", args).AssertOutput(`{}`)

	// 4. Get again.
	mcp.Call("password_get", args).AssertToolError(errPasswordNotFound)
	api.Get(path).AssertProblemType(passwordNotFound)
	assertSessionsEnded(session)
}
