// End-to-end tests of the API key endpoints: each test drives the whole app through /api/v1, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps in order,
// each with the result it expects; the numbered comments in the test mark where each step starts.
package apikey_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/envelope"
	apikeyapi "pkgsilo/feature/apikey/api"
	groupapi "pkgsilo/feature/group/api"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/apptest"
)

// The problem types of the API key endpoints, written out independently of the code: clients tell
// problems apart by them, so a changed slug, title or status must fail these tests.
var (
	apiKeyNotFound   = envelope.DefineProblemType("api-key-not-found", "API key not found", http.StatusNotFound)
	identityNotFound = envelope.DefineProblemType("identity-not-found", "Identity not found", http.StatusNotFound)
	fieldsDenied     = envelope.DefineProblemType("fields-denied", "Fields denied", http.StatusForbidden)
	forbidden        = envelope.ProblemType{Type: "about:blank", Title: "Forbidden", Status: http.StatusForbidden}
)

const identitiesPath = "/api/v1/identities"

const groupsPath = "/api/v1/groups"

type (
	apiKey        = apikeyapi.KeyResponse
	createdAPIKey = apikeyapi.CreatedKeyResponse
	identity      = identityapi.IdentityResponse
	group         = groupapi.GroupResponse
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

// keysPathOf returns the path of the API keys of the identity with the given ID.
func keysPathOf(id uuid.UUID) string {
	return identitiesPath + "/" + id.String() + "/api-keys"
}

// keyIDs returns the IDs of the API keys at keysPath that the client may read, in order.
func keyIDs(api *apitest.Client, keysPath string) []uuid.UUID {
	keys := listKeys(api, keysPath)
	ids := make([]uuid.UUID, len(keys))
	for i, key := range keys {
		ids[i] = key.ID
	}

	return ids
}

// listKeys returns the API keys at keysPath that the client may read.
func listKeys(api *apitest.Client, keysPath string) []apiKey {
	return apitest.DecodeCollection[apiKey](api.Get(keysPath), http.StatusOK).Data
}

// createGroup creates a group with the given name and the identities as members, and returns its ID.
func createGroup(api *apitest.Client, name string, members ...uuid.UUID) uuid.UUID {
	created := apitest.DecodeResource[group](api.Post(groupsPath, `{"name":"`+name+`"}`), http.StatusCreated)
	for _, member := range members {
		api.Put(groupsPath+"/"+created.ID.String()+"/members/"+member.String(), "").AssertNoContent()
	}

	return created.ID
}

// addPolicy creates a policy with the given source, under a name of its own, through the client.
func addPolicy(t *testing.T, api *apitest.Client, source string) {
	t.Helper()

	body, err := json.Marshal(map[string]string{"name": uuid.NewV4().String(), "source": source})
	require.NoError(t, err)
	api.Post("/api/v1/policies", string(body)).AssertStatus(http.StatusCreated)
}

// assertFieldsDenied asserts a fields-denied problem that points to the given members of the request body,
// in that order.
func assertFieldsDenied(t *testing.T, resp *apitest.Response, pointers ...string) {
	t.Helper()
	resp.AssertProblemType(fieldsDenied)

	var problem struct {
		Errors []envelope.ValidationError `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &problem))
	got := make([]string, len(problem.Errors))
	for i, err := range problem.Errors {
		got[i] = err.Pointer
	}
	assert.Equal(t, pointers, got)
}

// assertLikeMissing asserts that send answers target with the same problem, to the byte, as missingTarget,
// which refers to something that doesn't exist.
func assertLikeMissing(
	t *testing.T, send func(target string) *apitest.Response, target, missingTarget string, want envelope.ProblemType,
) {
	t.Helper()

	hidden := send(target)
	hidden.AssertProblemType(want)
	assert.Equal(t, send(missingTarget).Body.String(), hidden.Body.String(), "%s must look like it's missing", target)
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
	assert.Equal(t, envelope.Visible(ada.ID), created.IdentityID)
	assert.Equal(t, envelope.Visible("ci"), created.Name)
	assert.Regexp(t, `^psk_[A-Z2-7]{52}$`, created.Key.Value)
	assert.Equal(t, envelope.Visible(created.Key.Value[len(created.Key.Value)-4:]), created.KeySuffix)
	assert.Equal(t, expiresAt, *created.ExpiresAt.Value)
	assert.Equal(t, envelope.Visible[*time.Time](nil), created.LastUsedAt)
	assert.WithinDuration(t, time.Now(), created.CreatedAt.Value, time.Minute)

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
	assert.Equal(t, envelope.Visible[*time.Time](nil), created.ExpiresAt)

	// 2. Get.
	got := apitest.DecodeResource[apiKey](api.Get(keysPath+"/"+created.ID.String()), http.StatusOK)
	assert.Equal(t, envelope.Visible[*time.Time](nil), got.ExpiresAt)
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
	assert.NotEqual(t, first.Key.Value, second.Key.Value)

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

// TestAPIKeys_ManageOwn checks what the seeded policies give an identity outside the administrators group on
// its own API keys, which is managing them:
//
//  1. Create ada with an API key: 201.
//  2. As ada, create an API key of hers that expires, and one that doesn't: 201 with the key for both,
//     attributed to her.
//  3. As ada, list her API keys: the three of them. Get the one that expires: 200.
//  4. As ada, delete the one that expires: 204. List her API keys: the other two.
//  5. Use the API key that doesn't expire: it authenticates as ada.
func TestAPIKeys_ManageOwn(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create ada.
	ada, asAda := apptest.NewIdentity(api, "ada")
	keysPath := keysPathOf(ada.ID)
	initial := keyIDs(api, keysPath)
	require.Len(t, initial, 1)

	// 2. Create her API keys.
	expiresAt := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	expiring := apitest.DecodeResource[createdAPIKey](
		asAda.Post(keysPath, `{"name":"ci","expiresAt":"`+expiresAt.Format(time.RFC3339Nano)+`"}`), http.StatusCreated)
	lasting := createKey(asAda, keysPath, "laptop")
	assert.Equal(t, envelope.Visible(ada.ID), expiring.CreatedBy)
	assert.Equal(t, envelope.Visible(ada.ID), lasting.CreatedBy)

	// 3. Read them.
	assert.Equal(t, []uuid.UUID{initial[0], expiring.ID, lasting.ID}, keyIDs(asAda, keysPath))
	expiringPath := keysPath + "/" + expiring.ID.String()
	assert.Equal(t, expiring.KeyResponse, apitest.DecodeResource[apiKey](asAda.Get(expiringPath), http.StatusOK))

	// 4. Delete one.
	asAda.Delete(expiringPath).AssertNoContent()
	assert.Equal(t, []uuid.UUID{initial[0], lasting.ID}, keyIDs(asAda, keysPath))

	// 5. Use the other.
	asAda.WithHeader("X-API-Key", lasting.Key.Value).Get(identitiesPath + "/" + ada.ID.String()).
		AssertStatus(http.StatusOK)
}

// TestAPIKeys_OfHiddenIdentity checks that ada, by the seeded policies, can't use the API keys of grace,
// whom she can't read, and can't tell grace from an identity that doesn't exist:
//
//  1. Create ada and grace with an API key each: 201.
//  2. As ada, list, create, get and delete grace's API keys: the same 404 identity-not-found, to the byte,
//     as for an identity that doesn't exist, with the same API key ID.
//  3. As admin, list grace's API keys: nothing was written.
func TestAPIKeys_OfHiddenIdentity(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	_, asAda := apptest.NewIdentity(api, "ada")
	grace, _ := apptest.NewIdentity(api, "grace")
	graceKeysPath := keysPathOf(grace.ID)
	graceKeys := listKeys(api, graceKeysPath)
	require.Len(t, graceKeys, 1)
	missingKeysPath := keysPathOf(uuid.NewV4())
	keyID := "/" + graceKeys[0].ID.String()

	// 2. Use grace's API keys.
	assertLikeMissing(t, asAda.Get, graceKeysPath, missingKeysPath, identityNotFound)
	assertLikeMissing(t, func(target string) *apitest.Response { return asAda.Post(target, `{"name":"mine"}`) },
		graceKeysPath, missingKeysPath, identityNotFound)
	assertLikeMissing(t, asAda.Get, graceKeysPath+keyID, missingKeysPath+keyID, identityNotFound)
	assertLikeMissing(t, asAda.Delete, graceKeysPath+keyID, missingKeysPath+keyID, identityNotFound)

	// 3. Nothing written.
	assert.Equal(t, graceKeys, listKeys(api, graceKeysPath))
}

// TestAPIKeys_ReadOnlyAudit checks a policy that limits access by method: the members of an audit group read
// all API keys, and write none but their own.
//
//  1. Create ada and grace with an API key each, and the group auditors with ada as a member: 201 and 204.
//  2. Add a policy that lets auditors read everything: 201.
//  3. As ada, list grace's API keys: the same as admin, with every field. Get grace's API key: 200 with
//     every field.
//  4. As ada, delete grace's API key: 403 Forbidden. Create an API key of grace with an expiry: 403
//     fields-denied pointing to name and expiresAt.
//  5. As admin, list grace's API keys: nothing was written.
func TestAPIKeys_ReadOnlyAudit(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace, _ := apptest.NewIdentity(api, "grace")
	auditors := createGroup(api, "auditors", ada.ID)
	graceKeysPath := keysPathOf(grace.ID)

	// 2. Add the policy.
	addPolicy(t, api, `permit (principal in Group::"`+auditors.String()+`", action in Action::"read", resource);`)

	// 3. Read.
	graceKeys := listKeys(api, graceKeysPath)
	require.Len(t, graceKeys, 1)
	assert.Equal(t, graceKeys, listKeys(asAda, graceKeysPath))
	graceKeyPath := graceKeysPath + "/" + graceKeys[0].ID.String()
	assert.Equal(t, graceKeys[0], apitest.DecodeResource[apiKey](asAda.Get(graceKeyPath), http.StatusOK))

	// 4. Write.
	asAda.Delete(graceKeyPath).AssertProblemType(forbidden)
	expiresAt := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	assertFieldsDenied(t, asAda.Post(graceKeysPath, `{"name":"mine","expiresAt":"`+expiresAt+`"}`),
		"#/name", "#/expiresAt")

	// 5. Nothing written.
	assert.Equal(t, graceKeys, listKeys(api, graceKeysPath))
}

// TestAPIKeys_ForbidOneKey checks that a list checks each API key as well as their identity, so that a forbid
// on one API key hides it, and that the pages are cut out of the readable API keys:
//
//  1. Create ada with an API key, and three more API keys of hers: 201.
//  2. Add a policy that forbids everything on her second API key: 201.
//  3. As ada, list her API keys two at a time: the first and the third, then the fourth alone. Only the
//     last page is short.
//  4. As ada, get and delete the second API key: the same 404 api-key-not-found, to the byte, as for an API
//     key that doesn't exist.
func TestAPIKeys_ForbidOneKey(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	keysPath := keysPathOf(ada.ID)
	first := keyIDs(api, keysPath)[0]
	second, third, fourth := createKey(api, keysPath, "second"), createKey(api, keysPath, "third"),
		createKey(api, keysPath, "fourth")

	// 2. Add the policy.
	addPolicy(t, api, `forbid (principal, action, resource == APIKey::"`+second.ID.String()+`");`)

	// 3. List.
	page := apitest.DecodeCollection[apiKey](asAda.Get(keysPath+"?limit=2"), http.StatusOK)
	if assert.Len(t, page.Data, 2) {
		assert.Equal(t, first, page.Data[0].ID)
		assert.Equal(t, third.KeyResponse, page.Data[1])
	}
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 2, NextOffset: new(2)}, page.Pagination)
	page = apitest.DecodeCollection[apiKey](asAda.Get(keysPath+"?limit=2&offset=2"), http.StatusOK)
	assert.Equal(t, []apiKey{fourth.KeyResponse}, page.Data)
	assert.Equal(t, envelope.Pagination{Offset: 2, Limit: 2}, page.Pagination)

	// 4. Use the second.
	missingPath := keysPath + "/" + uuid.NewV4().String()
	assertLikeMissing(t, asAda.Get, keysPath+"/"+second.ID.String(), missingPath, apiKeyNotFound)
	assertLikeMissing(t, asAda.Delete, keysPath+"/"+second.ID.String(), missingPath, apiKeyNotFound)
}

// TestAPIKeys_OwnerGroups checks that an API key's owner carries its groups, so that a policy on the groups of
// the owner matches:
//
//  1. Create ada with an API key, grace and carol with an API key each, and the group secret with grace as a
//     member: 201 and 204.
//  2. Add a policy that lets ada read everything, but not the API keys of the members of secret: 201.
//  3. As ada, list grace's API keys: an empty 200. List carol's: her API key.
func TestAPIKeys_OwnerGroups(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace, _ := apptest.NewIdentity(api, "grace")
	carol, _ := apptest.NewIdentity(api, "carol")
	secret := createGroup(api, "secret", grace.ID)

	// 2. Add the policy.
	addPolicy(t, api, `
		permit (principal == Identity::"`+ada.ID.String()+`", action in Action::"read", resource);
		forbid (principal, action, resource is APIKey) when { resource.owner in Group::"`+secret.String()+`" };
	`)

	// 3. List.
	assert.Empty(t, listKeys(asAda, keysPathOf(grace.ID)))
	assert.Equal(t, listKeys(api, keysPathOf(carol.ID)), listKeys(asAda, keysPathOf(carol.ID)))
}

// TestAPIKeys_CreateFields checks that creating an API key checks only the fields that the request sets:
//
//  1. Create ada with an API key: 201.
//  2. Add a policy that forbids ada to set the expiry of an API key: 201.
//  3. As ada, create an API key of hers with an expiry: 403 fields-denied pointing to expiresAt.
//  4. As ada, create one without an expiry: 201, and the API key never expires.
func TestAPIKeys_CreateFields(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create ada.
	ada, asAda := apptest.NewIdentity(api, "ada")
	keysPath := keysPathOf(ada.ID)

	// 2. Add the policy.
	addPolicy(t, api, `forbid (principal, action == Action::"apikey:create:expiresAt", resource);`)

	// 3. Create with an expiry.
	expiresAt := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	assertFieldsDenied(t, asAda.Post(keysPath, `{"name":"ci","expiresAt":"`+expiresAt+`"}`), "#/expiresAt")

	// 4. Create without an expiry.
	assert.Equal(t, envelope.Visible[*time.Time](nil), createKey(asAda, keysPath, "ci").ExpiresAt)
}

// TestAPIKeys_CreateUnreadable checks that a new API key's key is returned even to a caller that can't read
// the API key, since it's the only time it's available:
//
//  1. Create ada with an API key: 201.
//  2. Add a policy that forbids ada to read API keys: 201.
//  3. As ada, create an API key of hers: 201 with its ID and key alone, at its Location.
//  4. As ada, get it: 404 api-key-not-found. Use its key: it authenticates as ada.
func TestAPIKeys_CreateUnreadable(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create ada.
	ada, asAda := apptest.NewIdentity(api, "ada")
	keysPath := keysPathOf(ada.ID)

	// 2. Add the policy.
	addPolicy(t, api, `forbid (principal, action == Action::"apikey:read", resource);`)

	// 3. Create.
	resp := asAda.Post(keysPath, `{"name":"ci"}`)
	created := apitest.DecodeResource[createdAPIKey](resp, http.StatusCreated)
	assert.Regexp(t, `^psk_[A-Z2-7]{52}$`, created.Key.Value)
	resp.AssertCreated(keysPath+"/"+created.ID.String(), `{"id":"`+created.ID.String()+`","key":"`+created.Key.Value+`"}`)

	// 4. Use it.
	asAda.Get(keysPath + "/" + created.ID.String()).AssertProblemType(apiKeyNotFound)
	asAda.WithHeader("X-API-Key", created.Key.Value).Get(identitiesPath + "/" + ada.ID.String()).
		AssertStatus(http.StatusOK)
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
	key := createKey(api, keysPath, "ci").Key.Value

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
	api.Put(groupsPath+"/"+apptest.AdministratorsGroup(t, api).String()+"/members/"+ada.ID.String(), "").
		AssertNoContent()
	key := createKey(api, adaKeysPath, "ci")
	assert.Equal(t, envelope.Visible[*time.Time](nil), key.LastUsedAt)
	asAda := api.WithHeader("X-API-Key", key.Key.Value)

	// 2. Create grace.
	grace, graceKeysPath := createIdentity(asAda, "grace")
	assert.Equal(t, envelope.Visible(ada.ID), grace.CreatedBy)

	// 3. Replace grace.
	replaced := apitest.DecodeResource[identity](asAda.Put(identitiesPath+"/"+grace.ID.String(),
		`{"username":"grace","displayName":"Grace Hopper"}`), http.StatusOK)
	assert.Equal(t, envelope.Visible(ada.ID), replaced.UpdatedBy)

	// 4. Create a key of grace.
	graceKey := createKey(asAda, graceKeysPath, "ci")
	assert.Equal(t, envelope.Visible(ada.ID), graceKey.CreatedBy)
	assert.Equal(t, envelope.Visible(grace.ID), graceKey.IdentityID)

	// 5. Get ada's key.
	used := apitest.DecodeResource[apiKey](api.Get(adaKeysPath+"/"+key.ID.String()), http.StatusOK)
	if assert.NotNil(t, used.LastUsedAt.Value) {
		assert.WithinDuration(t, time.Now(), *used.LastUsedAt.Value, time.Minute)
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
	api.WithHeader("X-API-Key", first.Key.Value).Get(identitiesPath).AssertStatus(http.StatusOK)
	api.WithHeader("X-API-Key", second.Key.Value).Get(identitiesPath).AssertStatus(http.StatusOK)

	// 2. Delete the first.
	api.Delete(keysPath + "/" + first.ID.String()).AssertNoContent()

	// 3. Use both.
	assertUnauthorized(t, api.WithHeader("X-API-Key", first.Key.Value).Get(identitiesPath))
	api.WithHeader("X-API-Key", second.Key.Value).Get(identitiesPath).AssertStatus(http.StatusOK)
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
	asAda := api.WithHeader("X-API-Key", createKey(api, keysPath, "ci").Key.Value)
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
	asAda := api.WithHeader("X-API-Key", created.Key.Value)
	asAda.Get(identitiesPath).AssertStatus(http.StatusOK)

	// 2. Wait for the expiry.
	assert.Eventually(t, func() bool {
		return asAda.Get(identitiesPath).Code == http.StatusUnauthorized
	}, 30*time.Second, 100*time.Millisecond)
	assertUnauthorized(t, asAda.Get(identitiesPath))

	// 3. Get the key.
	got := apitest.DecodeResource[apiKey](api.Get(keysPath+"/"+created.ID.String()), http.StatusOK)
	if assert.NotNil(t, got.LastUsedAt.Value) {
		assert.True(t, got.LastUsedAt.Value.Before(expiresAt))
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
	assert.Equal(t, envelope.Visible("admin"), admin.Username)
	assert.Equal(t, envelope.Visible(uuid.Nil()), admin.CreatedBy)

	// 2. List admin's keys.
	keysPath := identitiesPath + "/" + admin.ID.String() + "/api-keys"
	keys := apitest.DecodeCollection[apiKey](api.Get(keysPath), http.StatusOK).Data
	require.Len(t, keys, 1)
	bootstrap := keys[0]
	assert.Equal(t, envelope.Visible("bootstrap"), bootstrap.Name)
	assert.Equal(t, envelope.Visible[*time.Time](nil), bootstrap.ExpiresAt)
	assert.NotNil(t, bootstrap.LastUsedAt.Value)
	assert.Equal(t, envelope.Visible(uuid.Nil()), bootstrap.CreatedBy)

	// 3. Create a key.
	own := createKey(api, keysPath, "own")
	assert.Equal(t, envelope.Visible(admin.ID), own.CreatedBy)

	// 4. Delete the bootstrap key.
	asOwn := api.WithHeader("X-API-Key", own.Key.Value)
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

// TestAPIKeys_HiddenExpiry checks that a hidden expiry is absent, unlike the null of an API key that never
// expires:
//
//  1. Create ada with an API key that never expires: 201.
//  2. As ada, get it: 200 with expiresAt null.
//  3. Add a policy that forbids ada to read the expiry of API keys: 201.
//  4. As ada, get it: 200 without expiresAt, and with every other field. As admin, get it: 200 with
//     expiresAt null.
func TestAPIKeys_HiddenExpiry(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	keyPath := keysPathOf(ada.ID) + "/" + keyIDs(api, keysPathOf(ada.ID))[0].String()

	// 2. Get it.
	visible := apitest.DecodeResource[map[string]any](asAda.Get(keyPath), http.StatusOK)
	assert.Contains(t, visible, "expiresAt")
	assert.Nil(t, visible["expiresAt"])

	// 3. Add the policy.
	addPolicy(t, api, `forbid (
		principal == Identity::"`+ada.ID.String()+`",
		action == Action::"apikey:read:expiresAt",
		resource
	);`)

	// 4. Get it again.
	hidden := apitest.DecodeResource[map[string]any](asAda.Get(keyPath), http.StatusOK)
	assert.NotContains(t, hidden, "expiresAt")
	delete(visible, "expiresAt")
	assert.Equal(t, visible, hidden, "every other field must stay visible")
	assert.Equal(t, envelope.Visible[*time.Time](nil),
		apitest.DecodeResource[apiKey](api.Get(keyPath), http.StatusOK).ExpiresAt)
}
