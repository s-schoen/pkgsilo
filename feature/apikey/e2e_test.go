// End-to-end tests of the API key endpoints: each test drives the whole app through /api/v1, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps in order,
// each with the result it expects; the numbered comments in the test mark where each step starts.
package apikey_test

import (
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"

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

type (
	apiKey        = apikeyapi.KeyResponse
	createdAPIKey = apikeyapi.CreatedKeyResponse
	identity      = identityapi.IdentityResponse
)

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
