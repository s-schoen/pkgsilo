// End-to-end tests of the API key endpoints: each test drives the whole app through /api/v1, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps in order,
// each with the result it expects; the numbered comments in the test mark where each step starts.
package apikey_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/envelope"
	apikeyapi "pkgsilo/feature/apikey/api"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/apptest"
)

// The problem types of the API key endpoints, written out independently of the code: clients tell
// problems apart by them, so a changed slug, title or status must fail these tests.
var (
	apiKeyNotFound   = envelope.DefineProblemType("api-key-not-found", "API key not found", http.StatusNotFound)
	identityNotFound = envelope.DefineProblemType("identity-not-found", "Identity not found", http.StatusNotFound)
)

const identitiesPath = "/api/v1/identities"

// administratorsPath is the path of the administrators group that the migrations seed, whose members the
// seeded policy allows everything.
const administratorsPath = "/api/v1/groups/7c1d3e52-8a4f-4b6e-9f21-5d0c8e7a3b14"

type (
	apiKey        = apikeyapi.KeyResponse
	createdAPIKey = apikeyapi.CreatedKeyResponse
	identity      = identityapi.IdentityResponse
)

// assertUnauthorized asserts the answer to a request without a valid credential, which doesn't say why
// it's rejected and tells which method is expected.
func assertUnauthorized(t *testing.T, resp *apitest.Response) {
	t.Helper()

	resp.AssertProblem(envelope.UnauthorizedProblem.New(envelope.Occurrence{}))
	assert.Equal(t, []string{`ApiKey realm="pkgsilo"`}, resp.Header().Values("WWW-Authenticate"))
}

// createIdentity creates an identity with the given username and returns it with the path of its API
// keys.
func createIdentity(api *apitest.Client, username string) (created identity, keysPath string) {
	resp := api.Post(identitiesPath, `{"username":"`+username+`","displayName":"`+username+`"}`)
	created = apitest.DecodeResource[identity](resp, http.StatusCreated)

	return created, identitiesPath + "/" + created.ID.String() + "/api-keys"
}

// createKey creates an API key at keysPath with the given name.
func createKey(api *apitest.Client, keysPath, name string) createdAPIKey {
	return apitest.DecodeResource[createdAPIKey](api.Post(keysPath, `{"name":"`+name+`"}`), http.StatusCreated)
}

// TestAPIKeys_Lifecycle takes one API key of ada through every endpoint:
//
//  1. Create ada, then an API key of hers that expires: 201 with the key, its suffix and its fields
//     set by the server, its Location under her API keys, and a response that mustn't be cached.
//  2. Get it through the Location: the created API key, without the key.
//  3. List her API keys: that API key alone, without the key.
//  4. Delete it: 204.
//  5. Get and delete it again: 404 api-key-not-found for both.
//  6. List her API keys: none.
func TestAPIKeys_Lifecycle(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, keysPath := createIdentity(api, "ada")
	expiresAt := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	resp := api.Post(keysPath, `{"name":"ci","expiresAt":"`+expiresAt.Format(time.RFC3339Nano)+`"}`)
	created := apitest.DecodeResource[createdAPIKey](resp, http.StatusCreated)
	location := keysPath + "/" + created.ID.String()
	assert.Equal(t, location, resp.Header().Get("Location"))
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
	assert.Equal(t, ada.ID, created.IdentityID)
	assert.Equal(t, "ci", created.Name)
	assert.Regexp(t, `^psk_[A-Z2-7]{52}$`, created.Key)
	assert.Equal(t, created.Key[len(created.Key)-4:], created.KeySuffix)
	assert.Equal(t, expiresAt, *created.ExpiresAt)
	assert.Nil(t, created.LastUsedAt)
	assert.WithinDuration(t, time.Now(), created.CreatedAt, time.Minute)

	// 2. Get. Decoding fails on a key in the response, which has no member for it.
	assert.Equal(t, created.KeyResponse, apitest.DecodeResource[apiKey](api.Get(location), http.StatusOK))

	// 3. List.
	listed := apitest.DecodeCollection[apiKey](api.Get(keysPath), http.StatusOK)
	assert.Equal(t, []apiKey{created.KeyResponse}, listed.Data)

	// 4. Delete.
	api.Delete(location).AssertNoContent()

	// 5. Get and delete again.
	api.Get(location).AssertProblemType(apiKeyNotFound)
	api.Delete(location).AssertProblemType(apiKeyNotFound)

	// 6. List again.
	assert.Empty(t, apitest.DecodeCollection[apiKey](api.Get(keysPath), http.StatusOK).Data)
}

// TestAPIKeys_NeverExpires checks that an API key without an expiry says so:
//
//  1. Create ada, then an API key of hers without an expiry: 201 with no expiry.
//  2. Get it: still no expiry.
func TestAPIKeys_NeverExpires(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	_, keysPath := createIdentity(api, "ada")
	created := createKey(api, keysPath, "ci")
	assert.Nil(t, created.ExpiresAt)

	// 2. Get.
	got := apitest.DecodeResource[apiKey](api.Get(keysPath+"/"+created.ID.String()), http.StatusOK)
	assert.Nil(t, got.ExpiresAt)
}

// TestAPIKeys_NamesNeedNotBeUnique checks that rotating an API key can keep its name:
//
//  1. Create ada, then two API keys of hers with the same name: 201 for both, with different keys.
//  2. List her API keys: both, in the order of their creation.
func TestAPIKeys_NamesNeedNotBeUnique(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	_, keysPath := createIdentity(api, "ada")
	first, second := createKey(api, keysPath, "ci"), createKey(api, keysPath, "ci")
	assert.NotEqual(t, first.Key, second.Key)

	// 2. List.
	listed := apitest.DecodeCollection[apiKey](api.Get(keysPath), http.StatusOK)
	assert.Equal(t, []apiKey{first.KeyResponse, second.KeyResponse}, listed.Data)
}

// TestAPIKeys_ListPages checks the pagination of an identity's API keys, which leaves out those of
// other identities:
//
//  1. Create ada with two API keys, and grace with one.
//  2. List one of ada's API keys: the first, and that another follows.
//  3. List the next: the second, and that none follows.
func TestAPIKeys_ListPages(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	_, keysPath := createIdentity(api, "ada")
	first, second := createKey(api, keysPath, "first"), createKey(api, keysPath, "second")
	_, graceKeysPath := createIdentity(api, "grace")
	createKey(api, graceKeysPath, "other")

	// 2. List the first page.
	page := apitest.DecodeCollection[apiKey](api.Get(keysPath+"?limit=1"), http.StatusOK)
	assert.Equal(t, []apiKey{first.KeyResponse}, page.Data)
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 1, NextOffset: new(1)}, page.Pagination)

	// 3. List the next page.
	page = apitest.DecodeCollection[apiKey](api.Get(keysPath+"?limit=1&offset=1"), http.StatusOK)
	assert.Equal(t, []apiKey{second.KeyResponse}, page.Data)
	assert.Equal(t, envelope.Pagination{Offset: 1, Limit: 1}, page.Pagination)
}

// TestAPIKeys_IdentityNotFound checks that every endpoint reports a missing identity in the way of
// the identity endpoints, whether the identity is unknown or was deleted:
//
//  1. Create ada with an API key, and delete her: 204, which deletes the API key with her.
//  2. List, create, get and delete an API key of hers or of an unknown identity: 404
//     identity-not-found for all of them.
func TestAPIKeys_IdentityNotFound(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create and delete ada.
	ada, adaKeysPath := createIdentity(api, "ada")
	key := createKey(api, adaKeysPath, "ci")
	api.Delete(identitiesPath + "/" + ada.ID.String()).AssertNoContent()

	// 2. Use the API keys of the missing identities.
	for name, keysPath := range map[string]string{
		"deleted": adaKeysPath,
		"unknown": identitiesPath + "/" + uuid.NewV4().String() + "/api-keys",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			api := api.For(t)
			keyPath := keysPath + "/" + key.ID.String()

			api.Get(keysPath).AssertProblemType(identityNotFound)
			api.Post(keysPath, `{"name":"ci"}`).AssertProblemType(identityNotFound)
			api.Get(keyPath).AssertProblemType(identityNotFound)
			api.Delete(keyPath).AssertProblemType(identityNotFound)
		})
	}
}

// TestAPIKeys_OfAnotherIdentity checks that an API key can only be reached through its own identity:
//
//  1. Create ada with an API key, and grace.
//  2. Get and delete the API key through grace: 404 api-key-not-found for both.
//  3. Get it through ada: it still exists.
func TestAPIKeys_OfAnotherIdentity(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	_, adaKeysPath := createIdentity(api, "ada")
	key := createKey(api, adaKeysPath, "ci")
	_, graceKeysPath := createIdentity(api, "grace")

	// 2. Get and delete through grace.
	api.Get(graceKeysPath + "/" + key.ID.String()).AssertProblemType(apiKeyNotFound)
	api.Delete(graceKeysPath + "/" + key.ID.String()).AssertProblemType(apiKeyNotFound)

	// 3. Get through ada.
	api.Get(adaKeysPath + "/" + key.ID.String()).AssertStatus(http.StatusOK)
}

// TestAuthentication_Required sends requests without a credential:
//
//  1. To every route of the identities, their API keys and the groups: 401 with the challenge.
//  2. To the identities again, as the first start's admin: only admin exists, so nothing was created.
func TestAuthentication_Required(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	id := uuid.NewV4().String()

	// 1. Send requests without a credential.
	for _, route := range []struct{ method, target string }{
		{http.MethodGet, identitiesPath},
		{http.MethodPost, identitiesPath},
		{http.MethodGet, identitiesPath + "/" + id},
		{http.MethodPut, identitiesPath + "/" + id},
		{http.MethodDelete, identitiesPath + "/" + id},
		{http.MethodGet, identitiesPath + "/" + id + "/api-keys"},
		{http.MethodPost, identitiesPath + "/" + id + "/api-keys"},
		{http.MethodGet, identitiesPath + "/" + id + "/api-keys/" + id},
		{http.MethodDelete, identitiesPath + "/" + id + "/api-keys/" + id},
		{http.MethodGet, "/api/v1/groups"},
		{http.MethodPost, "/api/v1/groups"},
		{http.MethodGet, "/api/v1/groups/" + id},
		{http.MethodPut, "/api/v1/groups/" + id},
		{http.MethodDelete, "/api/v1/groups/" + id},
		{http.MethodGet, "/api/v1/groups/" + id + "/members"},
		{http.MethodPut, "/api/v1/groups/" + id + "/members/" + id},
		{http.MethodDelete, "/api/v1/groups/" + id + "/members/" + id},
	} {
		body := ""
		if route.method == http.MethodPost || route.method == http.MethodPut {
			body = `{"username":"ada","displayName":"Ada","name":"ada"}`
		}
		assertUnauthorized(t, api.Anonymous().Send(route.method, route.target, body))
	}

	// 2. List the identities as admin.
	assert.Len(t, apitest.DecodeCollection[identity](api.Get(identitiesPath), http.StatusOK).Data, 1)
}

// TestAuthentication_RejectedKeys sends requests with keys that aren't any API key's, all of which get
// the same 401 with the challenge, whatever the reason.
func TestAuthentication_RejectedKeys(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	_, keysPath := createIdentity(api, "ada")
	key := createKey(api, keysPath, "ci").Key

	for name, rejected := range map[string]string{
		"without the prefix": "not-a-key",
		"only the prefix":    "psk_",
		"unknown":            "psk_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"altered":            key + "A",
		"lowercased":         strings.ToLower(key),
		"without the suffix": key[:len(key)-4],
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertUnauthorized(t, api.For(t).WithHeader("X-API-Key", rejected).Get(identitiesPath))
		})
	}
}

// TestAuthentication_KeyActsAsItsIdentity follows the requests of one key of ada:
//
//  1. Create ada as admin, add her to the administrators group, so that she may write identities and keys,
//     then create a key of hers: it has not been used.
//  2. Create grace with ada's key: 201, attributed to ada rather than admin.
//  3. Replace grace with ada's key: 200, the update attributed to ada.
//  4. Create a key of grace with ada's key: 201, attributed to ada, but belonging to grace.
//  5. Get ada's key: its last use is now recorded.
func TestAuthentication_KeyActsAsItsIdentity(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create ada and her key.
	ada, adaKeysPath := createIdentity(api, "ada")
	api.Put(administratorsPath+"/members/"+ada.ID.String(), "").AssertNoContent()
	key := createKey(api, adaKeysPath, "ci")
	assert.Nil(t, key.LastUsedAt)
	asAda := api.WithHeader("X-API-Key", key.Key)

	// 2. Create grace.
	grace, graceKeysPath := createIdentity(asAda, "grace")
	assert.Equal(t, ada.ID, grace.CreatedBy)

	// 3. Replace grace.
	replaced := apitest.DecodeResource[identity](asAda.Put(identitiesPath+"/"+grace.ID.String(),
		`{"username":"grace","displayName":"Grace Hopper"}`), http.StatusOK)
	assert.Equal(t, ada.ID, replaced.UpdatedBy)

	// 4. Create a key of grace.
	graceKey := createKey(asAda, graceKeysPath, "ci")
	assert.Equal(t, ada.ID, graceKey.CreatedBy)
	assert.Equal(t, grace.ID, graceKey.IdentityID)

	// 5. Get ada's key.
	used := apitest.DecodeResource[apiKey](api.Get(adaKeysPath+"/"+key.ID.String()), http.StatusOK)
	if assert.NotNil(t, used.LastUsedAt) {
		assert.WithinDuration(t, time.Now(), *used.LastUsedAt, time.Minute)
	}
}

// TestAuthentication_DeletedKeyStopsWorking checks that revoking one key takes effect at once and
// leaves the identity's others alone:
//
//  1. Create ada with two keys: both work.
//  2. Delete the first: 204.
//  3. Use both: 401 with the challenge for the first, and the second still works.
func TestAuthentication_DeletedKeyStopsWorking(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	_, keysPath := createIdentity(api, "ada")
	first, second := createKey(api, keysPath, "first"), createKey(api, keysPath, "second")
	api.WithHeader("X-API-Key", first.Key).Get(identitiesPath).AssertStatus(http.StatusOK)
	api.WithHeader("X-API-Key", second.Key).Get(identitiesPath).AssertStatus(http.StatusOK)

	// 2. Delete the first.
	api.Delete(keysPath + "/" + first.ID.String()).AssertNoContent()

	// 3. Use both.
	assertUnauthorized(t, api.WithHeader("X-API-Key", first.Key).Get(identitiesPath))
	api.WithHeader("X-API-Key", second.Key).Get(identitiesPath).AssertStatus(http.StatusOK)
}

// TestAuthentication_DeletedIdentityStopsWorking checks that the keys of an identity don't outlive it:
//
//  1. Create ada with a key: it works.
//  2. Delete ada: 204.
//  3. Use the key: 401 with the challenge.
func TestAuthentication_DeletedIdentityStopsWorking(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, keysPath := createIdentity(api, "ada")
	asAda := api.WithHeader("X-API-Key", createKey(api, keysPath, "ci").Key)
	asAda.Get(identitiesPath).AssertStatus(http.StatusOK)

	// 2. Delete ada.
	api.Delete(identitiesPath + "/" + ada.ID.String()).AssertNoContent()

	// 3. Use the key.
	assertUnauthorized(t, asAda.Get(identitiesPath))
}

// TestAuthentication_ExpiredKeyStopsWorking checks that a key works until its expiry only:
//
//  1. Create ada with a key that expires shortly: it works.
//  2. Wait for the expiry, then use the key: 401 with the challenge.
//  3. Get the key: it still exists, and its last use is from before the expiry.
func TestAuthentication_ExpiredKeyStopsWorking(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	_, keysPath := createIdentity(api, "ada")
	expiresAt := time.Now().Add(3 * time.Second).UTC()
	created := apitest.DecodeResource[createdAPIKey](
		api.Post(keysPath, `{"name":"ci","expiresAt":"`+expiresAt.Format(time.RFC3339Nano)+`"}`), http.StatusCreated)
	asAda := api.WithHeader("X-API-Key", created.Key)
	asAda.Get(identitiesPath).AssertStatus(http.StatusOK)

	// 2. Wait for the expiry.
	assert.Eventually(t, func() bool {
		return asAda.Get(identitiesPath).Code == http.StatusUnauthorized
	}, 30*time.Second, 100*time.Millisecond)
	assertUnauthorized(t, asAda.Get(identitiesPath))

	// 3. Get the key.
	got := apitest.DecodeResource[apiKey](api.Get(keysPath+"/"+created.ID.String()), http.StatusOK)
	if assert.NotNil(t, got.LastUsedAt) {
		assert.True(t, got.LastUsedAt.Before(expiresAt))
	}
}

// TestAuthentication_FirstStart checks what the first start leaves behind, and that its key can be
// replaced as its log message asks:
//
//  1. List the identities: admin alone, attributed to pkgsilo itself.
//  2. List admin's keys: bootstrap alone, which never expires, attributed to pkgsilo itself and last
//     used by these requests.
//  3. Create a key of admin with the bootstrap key: 201, attributed to admin.
//  4. Delete the bootstrap key with the new key: 204.
//  5. Use both: 401 with the challenge for the bootstrap key, and the new key still works.
func TestAuthentication_FirstStart(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. List the identities.
	identities := apitest.DecodeCollection[identity](api.Get(identitiesPath), http.StatusOK).Data
	require.Len(t, identities, 1)
	admin := identities[0]
	assert.Equal(t, "admin", admin.Username)
	assert.Equal(t, uuid.Nil(), admin.CreatedBy)

	// 2. List admin's keys.
	keysPath := identitiesPath + "/" + admin.ID.String() + "/api-keys"
	keys := apitest.DecodeCollection[apiKey](api.Get(keysPath), http.StatusOK).Data
	require.Len(t, keys, 1)
	bootstrap := keys[0]
	assert.Equal(t, "bootstrap", bootstrap.Name)
	assert.Nil(t, bootstrap.ExpiresAt)
	assert.NotNil(t, bootstrap.LastUsedAt)
	assert.Equal(t, uuid.Nil(), bootstrap.CreatedBy)

	// 3. Create a key.
	own := createKey(api, keysPath, "own")
	assert.Equal(t, admin.ID, own.CreatedBy)

	// 4. Delete the bootstrap key.
	asOwn := api.WithHeader("X-API-Key", own.Key)
	asOwn.Delete(keysPath + "/" + bootstrap.ID.String()).AssertNoContent()

	// 5. Use both.
	assertUnauthorized(t, api.Get(identitiesPath))
	asOwn.Get(identitiesPath).AssertStatus(http.StatusOK)
}

// TestAuthentication_RoutingProblems checks that authenticated requests that match no route still get
// the problems of the application router, past the authentication's route group:
//
//  1. Get an unknown path below the identities: 404 with the generic problem.
//  2. Patch the identities, which has no such method: 405 with the generic problem.
func TestAuthentication_RoutingProblems(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Get an unknown path.
	api.Get(identitiesPath + "/" + uuid.NewV4().String() + "/unknown").
		AssertProblem(envelope.NotFoundProblem.New(envelope.Occurrence{}))

	// 2. Patch the identities.
	api.Send(http.MethodPatch, identitiesPath, "").
		AssertProblem(envelope.MethodNotAllowedProblem.New(envelope.Occurrence{}))
}
