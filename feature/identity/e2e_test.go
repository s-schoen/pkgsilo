// End-to-end tests of the identity endpoints: each test drives the whole app through /api/v1, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps in order,
// each with the result it expects; the numbered comments in the test mark where each step starts.
package identity_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/envelope"
	groupapi "pkgsilo/feature/group/api"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/apptest"
)

// The problem types of the identity endpoints, written out independently of the code: clients tell
// problems apart by them, so a changed slug, title or status must fail these tests.
var (
	identityNotFound = envelope.DefineProblemType("identity-not-found", "Identity not found", http.StatusNotFound)
	usernameTaken    = envelope.DefineProblemType("username-taken", "Username taken", http.StatusConflict)
	fieldsDenied     = envelope.DefineProblemType("fields-denied", "Fields denied", http.StatusForbidden)
	invalidBody      = envelope.DefineProblemType("invalid-body", "Invalid request body", http.StatusUnprocessableEntity)
	forbidden        = envelope.ProblemType{Type: "about:blank", Title: "Forbidden", Status: http.StatusForbidden}
	badRequest       = envelope.ProblemType{Type: "about:blank", Title: "Bad Request", Status: http.StatusBadRequest}
	unsupportedMedia = envelope.ProblemType{
		Type: "about:blank", Title: "Unsupported Media Type", Status: http.StatusUnsupportedMediaType,
	}
)

const identitiesPath = "/api/v1/identities"

type identity = identityapi.IdentityResponse

// TestIdentities_Lifecycle takes one identity, ada, through every endpoint:
//
//  1. Create ada: 201 with the new identity, its Location under /api/v1/identities, and its
//     audit fields set by the server.
//  2. Get her through the Location: the created identity.
//  3. List the identities: admin, who the first start created and who made the request, then ada,
//     attributed to admin.
//  4. Replace her username and display name: 200 with the new values, the ID and creation time
//     kept and the update time advanced.
//  5. Get her again: the replaced identity, so the change was stored.
//  6. Delete her: 204.
//  7. Get and delete her again: 404 identity-not-found for both.
func TestIdentities_Lifecycle(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	created := api.Post(identitiesPath, `{"username":"ada","displayName":"Ada Lovelace"}`)
	ada := created.DecodeResource[identity](http.StatusCreated)
	location := identitiesPath + "/" + ada.ID.String()
	assert.Equal(t, location, created.Header().Get("Location"))
	assert.Equal(t, envelope.Visible("ada"), ada.Username)
	assert.Equal(t, envelope.Visible("Ada Lovelace"), ada.DisplayName)
	assert.Equal(t, ada.CreatedBy, ada.UpdatedBy)
	assert.WithinDuration(t, time.Now(), ada.CreatedAt.Value, time.Minute)
	assert.Equal(t, ada.CreatedAt, ada.UpdatedAt)

	// 2. Get.
	assert.Equal(t, ada, api.Get(location).DecodeResource[identity](http.StatusOK))

	// 3. List.
	listed := api.Get(identitiesPath).DecodeCollection[identity](http.StatusOK).Data
	require.Len(t, listed, 2)
	assert.Equal(t, envelope.Visible("admin"), listed[0].Username)
	assert.Equal(t, ada, listed[1])
	assert.Equal(t, envelope.Visible(listed[0].ID), ada.CreatedBy)

	// 4. Replace.
	replaced := api.Put(location, `{"username":"ada-lovelace","displayName":"Countess of Lovelace"}`).
		DecodeResource[identity](http.StatusOK)
	assert.Equal(t, ada.ID, replaced.ID)
	assert.Equal(t, envelope.Visible("ada-lovelace"), replaced.Username)
	assert.Equal(t, envelope.Visible("Countess of Lovelace"), replaced.DisplayName)
	assert.Equal(t, ada.CreatedAt, replaced.CreatedAt, "the creation time is immutable")
	assert.True(t, replaced.UpdatedAt.Value.After(ada.UpdatedAt.Value), "the update time must advance")

	// 5. Get again.
	assert.Equal(t, replaced, api.Get(location).DecodeResource[identity](http.StatusOK))

	// 6. Delete.
	api.Delete(location).AssertNoContent()

	// 7. Get and delete again.
	api.Get(location).AssertProblemType(identityNotFound)
	api.Delete(location).AssertProblemType(identityNotFound)
}

// TestIdentities_UsernameTaken checks that the database's unique username constraint reaches clients
// as a conflict:
//
//  1. Create ada and grace: 201 for both.
//  2. Create another identity named ada: 409 username-taken.
//  3. Rename grace to ada: 409 username-taken, and grace is unchanged.
//
// Steps 2 and 3 are parallel subtests on the same app.
func TestIdentities_UsernameTaken(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create ada and grace.
	api.Post(identitiesPath, `{"username":"ada","displayName":"Ada Lovelace"}`).AssertStatus(http.StatusCreated)
	grace := api.Post(identitiesPath, `{"username":"grace","displayName":"Grace Hopper"}`).
		DecodeResource[identity](http.StatusCreated)
	gracePath := identitiesPath + "/" + grace.ID.String()

	// 2. Create another ada.
	t.Run("create", func(t *testing.T) {
		t.Parallel()
		api := api.For(t)

		api.Post(identitiesPath, `{"username":"ada","displayName":"Another Ada"}`).AssertProblemType(usernameTaken)
	})
	// 3. Rename grace to ada.
	t.Run("replace", func(t *testing.T) {
		t.Parallel()
		api := api.For(t)

		api.Put(gracePath, `{"username":"ada","displayName":"Grace Hopper"}`).AssertProblemType(usernameTaken)
		assert.Equal(t, grace, api.Get(gracePath).DecodeResource[identity](http.StatusOK))
	})
}

// TestIdentities_ListPages pages through the identities, which are listed in creation order:
//
//  1. Create ada and grace, in that order: 201 for each.
//  2. List the first page of 2: admin, who the first start created before them, then ada, and a
//     nextOffset of 2.
//  3. List from offset 2: grace alone, on the last page, without a nextOffset.
func TestIdentities_ListPages(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	var created []identity
	for _, username := range []string{"ada", "grace"} {
		resp := api.Post(identitiesPath, `{"username":"`+username+`","displayName":"`+username+`"}`)
		created = append(created, resp.DecodeResource[identity](http.StatusCreated))
	}

	// 2. List the first page.
	first := api.Get(identitiesPath + "?limit=2").DecodeCollection[identity](http.StatusOK)
	require.Len(t, first.Data, 2)
	assert.Equal(t, envelope.Visible("admin"), first.Data[0].Username, "identities are listed in creation order")
	assert.Equal(t, created[0], first.Data[1])
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 2, NextOffset: new(2)}, first.Pagination)

	// 3. List the last page.
	last := api.Get(identitiesPath + "?offset=2&limit=2").DecodeCollection[identity](http.StatusOK)
	assert.Equal(t, created[1:], last.Data)
	assert.Equal(t, envelope.Pagination{Offset: 2, Limit: 2}, last.Pagination)
}

// addPolicy creates a policy with the given source, under a name of its own, through the client.
func addPolicy(t *testing.T, api *apitest.Client, source string) {
	t.Helper()

	body, err := json.Marshal(map[string]string{"name": uuid.NewV4().String(), "source": source})
	require.NoError(t, err)
	api.Post("/api/v1/policies", string(body)).AssertStatus(http.StatusCreated)
}

// insertPolicy stores a policy and announces it, like every policy write does. It's for a policy that the API
// rejects, as a database changed by hand may hold it.
func insertPolicy(t *testing.T, db *sqlx.DB, source string) {
	t.Helper()

	id := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO policies (id, name, source, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $1, $1, now(), now())`,
		id, id.String(), source,
	)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `UPDATE policy_revision SET revision = revision + 1`)
	require.NoError(t, err)
}

// TestIdentities_OwnIdentityOnly checks what the seeded policies give an identity outside the
// administrators group, which is reading itself alone:
//
//  1. Create ada and grace, each with an API key: 201 for each.
//  2. As ada, get ada: 200 with ada.
//  3. As ada, get grace: the same 404 identity-not-found, to the byte, as an identity that doesn't exist.
//  4. As ada, list the identities: ada alone, without a nextOffset, so that the admin and grace leave no
//     trace.
//  5. As ada, list from offset 1: empty, without a nextOffset.
//  6. As admin, get grace and list the identities: 200 with grace, and all three, so that the first start
//     has full access through the administrators group.
func TestIdentities_OwnIdentityOnly(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace, _ := apptest.NewIdentity(api, "grace")
	gracePath := identitiesPath + "/" + grace.ID.String()

	// 2. Get ada.
	assert.Equal(t, ada, asAda.Get(identitiesPath + "/" + ada.ID.String()).DecodeResource[identity](http.StatusOK))

	// 3. Get grace.
	hidden := asAda.Get(gracePath)
	hidden.AssertProblemType(identityNotFound)
	missing := asAda.Get(identitiesPath + "/" + uuid.NewV4().String())
	assert.Equal(t, missing.Body.String(), hidden.Body.String(), "a hidden identity must look like a missing one")

	// 4. List.
	listed := asAda.Get(identitiesPath).DecodeCollection[identity](http.StatusOK)
	assert.Equal(t, []identity{ada}, listed.Data)
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 50}, listed.Pagination)

	// 5. List from offset 1.
	past := asAda.Get(identitiesPath + "?offset=1").DecodeCollection[identity](http.StatusOK)
	assert.Empty(t, past.Data)
	assert.Equal(t, envelope.Pagination{Offset: 1, Limit: 50}, past.Pagination)

	// 6. As admin.
	assert.Equal(t, grace, api.Get(gracePath).DecodeResource[identity](http.StatusOK))
	all := api.Get(identitiesPath).DecodeCollection[identity](http.StatusOK).Data
	require.Len(t, all, 3)
	assert.Equal(t, []identity{ada, grace}, all[1:])
}

// TestIdentities_ListPagesWithHiddenRows pages through what a policy lets an identity read, with hidden
// identities in between, which must not show in the page sizes. It also shows that a policy applies on the
// next request and matches through the resource's groups:
//
//  1. Create the reader and identities one to six, and the group visible with two, four and six as
//     members: 201 and 204 for each.
//  2. As the reader, list the identities: the reader alone, since no policy lets it read more yet.
//  3. Add a policy that lets the reader read the members of visible.
//  4. List the first page of 2: the reader and two, and a nextOffset of 2, with hidden one between them.
//  5. List from offset 2: four and six, without a nextOffset, with hidden three and five around them.
//  6. Get three: 404 identity-not-found. Get four: 200.
func TestIdentities_ListPagesWithHiddenRows(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	reader, asReader := apptest.NewIdentity(api, "reader")
	created := map[string]identity{}
	for _, username := range []string{"one", "two", "three", "four", "five", "six"} {
		resp := api.Post(identitiesPath, `{"username":"`+username+`","displayName":"`+username+`"}`)
		created[username] = resp.DecodeResource[identity](http.StatusCreated)
	}
	group := api.Post("/api/v1/groups", `{"name":"visible"}`).DecodeResource[groupapi.GroupResponse](http.StatusCreated)
	for _, username := range []string{"two", "four", "six"} {
		api.Put("/api/v1/groups/"+group.ID.String()+"/members/"+created[username].ID.String(), "").AssertNoContent()
	}

	// 2. List before the policy.
	assert.Equal(t, []identity{reader},
		asReader.Get(identitiesPath).DecodeCollection[identity](http.StatusOK).Data)

	// 3. Add the policy.
	addPolicy(t, api, `permit (
		principal == Identity::"`+reader.ID.String()+`",
		action in Action::"read",
		resource in Group::"`+group.ID.String()+`"
	);`)

	// 4. List the first page.
	first := asReader.Get(identitiesPath + "?limit=2").DecodeCollection[identity](http.StatusOK)
	assert.Equal(t, []identity{reader, created["two"]}, first.Data)
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 2, NextOffset: new(2)}, first.Pagination)

	// 5. List the last page.
	last := asReader.Get(identitiesPath + "?offset=2&limit=2").DecodeCollection[identity](http.StatusOK)
	assert.Equal(t, []identity{created["four"], created["six"]}, last.Data)
	assert.Equal(t, envelope.Pagination{Offset: 2, Limit: 2}, last.Pagination)

	// 6. Get one by one.
	asReader.Get(identitiesPath + "/" + created["three"].ID.String()).AssertProblemType(identityNotFound)
	assert.Equal(t, created["four"],
		asReader.Get(identitiesPath + "/" + created["four"].ID.String()).DecodeResource[identity](http.StatusOK))
}

// TestIdentities_FailingPolicy checks that a policy that fails to evaluate results in an error rather than
// in access, even for the admin, whom another policy allows everything:
//
//  1. Add a forbid that fails to evaluate on every request.
//  2. Get the admin and list the identities: 500 for both, without details.
func TestIdentities_FailingPolicy(t *testing.T) {
	t.Parallel()
	api, db := apptest.NewWithDB(t)
	admin := api.Get(identitiesPath).DecodeCollection[identity](http.StatusOK).Data[0]

	// 1. Add the policy.
	insertPolicy(t, db, `forbid (principal, action, resource) when { resource.missing == principal };`)

	// 2. Get and list.
	api.Get(identitiesPath + "/" + admin.ID.String()).AssertInternalError()
	api.Get(identitiesPath).AssertInternalError()
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

// TestIdentities_WritesDeniedByDefault checks what an identity outside the administrators group may write
// with the seeded policies alone, which is nothing:
//
//  1. Create ada and grace, each with an API key: 201 for each.
//  2. As ada, create carol: 403 fields-denied pointing to username and displayName, and, as admin, no
//     carol in the list.
//  3. As ada, replace ada with her current values: 403 fields-denied pointing to both, even though nothing
//     would change, and ada is unchanged.
//  4. As ada, delete ada: 403 Forbidden, and ada still exists.
//  5. As ada, replace and delete grace, whom she can't read: the same 404 identity-not-found, to the byte,
//     as for an identity that doesn't exist, rather than a 403 that would reveal grace.
//  6. As ada, replace grace and an identity that doesn't exist with an invalid body: the same 422
//     invalid-body for both.
func TestIdentities_WritesDeniedByDefault(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	adaPath := identitiesPath + "/" + ada.ID.String()
	grace, _ := apptest.NewIdentity(api, "grace")
	gracePath := identitiesPath + "/" + grace.ID.String()
	missingPath := identitiesPath + "/" + uuid.NewV4().String()

	// 2. Create carol.
	assertFieldsDenied(t, asAda.Post(identitiesPath, `{"username":"carol","displayName":"Carol"}`),
		"#/username", "#/displayName")
	listed := api.Get(identitiesPath).DecodeCollection[identity](http.StatusOK).Data
	require.Len(t, listed, 3, "nothing may be created")
	assert.Equal(t, []identity{ada, grace}, listed[1:])

	// 3. Replace ada.
	assertFieldsDenied(t, asAda.Put(adaPath, `{"username":"ada","displayName":"ada"}`),
		"#/username", "#/displayName")
	assert.Equal(t, ada, api.Get(adaPath).DecodeResource[identity](http.StatusOK))

	// 4. Delete ada.
	asAda.Delete(adaPath).AssertProblemType(forbidden)
	assert.Equal(t, ada, api.Get(adaPath).DecodeResource[identity](http.StatusOK))

	// 5. Replace and delete grace.
	const body = `{"username":"grace","displayName":"grace"}`
	hidden := asAda.Put(gracePath, body)
	hidden.AssertProblemType(identityNotFound)
	assert.Equal(t, asAda.Put(missingPath, body).Body.String(), hidden.Body.String())
	hidden = asAda.Delete(gracePath)
	hidden.AssertProblemType(identityNotFound)
	assert.Equal(t, asAda.Delete(missingPath).Body.String(), hidden.Body.String())

	// 6. Replace with an invalid body.
	hidden = asAda.Put(gracePath, `{"username":"ab"}`)
	hidden.AssertProblemType(invalidBody)
	assert.Equal(t, asAda.Put(missingPath, `{"username":"ab"}`).Body.String(), hidden.Body.String())
}

// TestIdentities_UpdateSelf checks that a policy can let identities edit themselves, field by field, and
// that this doesn't let them create identities:
//
//  1. Create ada with an API key: 201.
//  2. Add a policy that lets every identity update itself.
//  3. As ada, replace ada: 200 with the new values.
//  4. As ada, create carol: 403 fields-denied pointing to username and displayName.
//  5. Add a policy that forbids changing usernames.
//  6. As ada, replace ada with a new display name and her username unchanged: 403 fields-denied pointing to
//     username alone, and ada is unchanged.
func TestIdentities_UpdateSelf(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	adaPath := identitiesPath + "/" + ada.ID.String()

	// 2. Add the update policy.
	addPolicy(t, api, `permit (principal, action in Action::"identity:update", resource)
		when { resource == principal };`)

	// 3. Replace ada.
	replaced := asAda.Put(adaPath, `{"username":"ada-lovelace","displayName":"Ada Lovelace"}`).
		DecodeResource[identity](http.StatusOK)
	assert.Equal(t, envelope.Visible("ada-lovelace"), replaced.Username)
	assert.Equal(t, envelope.Visible("Ada Lovelace"), replaced.DisplayName)
	assert.Equal(t, envelope.Visible(ada.ID), replaced.UpdatedBy)

	// 4. Create carol.
	assertFieldsDenied(t, asAda.Post(identitiesPath, `{"username":"carol","displayName":"Carol"}`),
		"#/username", "#/displayName")

	// 5. Add the forbid.
	addPolicy(t, api, `forbid (principal, action == Action::"identity:update:username", resource);`)

	// 6. Replace ada again.
	assertFieldsDenied(t, asAda.Put(adaPath, `{"username":"ada-lovelace","displayName":"Countess of Lovelace"}`),
		"#/username")
	assert.Equal(t, replaced, api.Get(adaPath).DecodeResource[identity](http.StatusOK))
}

// TestIdentities_CreateAndDeleteByPolicy checks that a policy can let an identity outside the
// administrators group create and delete identities, without letting it replace them:
//
//  1. Create ada with an API key: 201.
//  2. Add a policy that lets ada read, create and delete identities.
//  3. As ada, create carol: 201 with carol, attributed to ada, at her Location.
//  4. As ada, replace carol: 403 fields-denied pointing to username and displayName.
//  5. As ada, delete carol: 204, and, as admin, get her: 404 identity-not-found.
func TestIdentities_CreateAndDeleteByPolicy(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")

	// 2. Add the policy.
	addPolicy(t, api, `permit (
		principal == Identity::"`+ada.ID.String()+`",
		action in [Action::"read", Action::"identity:create", Action::"identity:delete"],
		resource is Identity
	);`)

	// 3. Create carol.
	created := asAda.Post(identitiesPath, `{"username":"carol","displayName":"Carol"}`)
	carol := created.DecodeResource[identity](http.StatusCreated)
	carolPath := identitiesPath + "/" + carol.ID.String()
	assert.Equal(t, carolPath, created.Header().Get("Location"))
	assert.Equal(t, envelope.Visible("carol"), carol.Username)
	assert.Equal(t, envelope.Visible(ada.ID), carol.CreatedBy)

	// 4. Replace carol.
	assertFieldsDenied(t, asAda.Put(carolPath, `{"username":"carol","displayName":"Carol"}`),
		"#/username", "#/displayName")

	// 5. Delete carol.
	asAda.Delete(carolPath).AssertNoContent()
	api.Get(carolPath).AssertProblemType(identityNotFound)
}

// TestIdentities_DisplayNameOnly checks a policy that limits access by attribute: an identity that may
// read the displayName of identities but nothing else of them sees their IDs and display names alone:
//
//  1. Create ada with an API key, and grace: 201 for each.
//  2. Add a policy that lets ada read identities and their displayName: 201.
//  3. As ada, get grace: 200 with her ID and display name, and no other member.
//  4. As ada, list the identities: admin and grace with their IDs and display names alone, and ada with
//     every field, which the seeded policy lets her read of herself.
func TestIdentities_DisplayNameOnly(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace, _ := apptest.NewIdentity(api, "grace")
	gracePath := identitiesPath + "/" + grace.ID.String()

	// 2. Add the policy.
	addPolicy(t, api, `permit (
		principal == Identity::"`+ada.ID.String()+`",
		action in [Action::"identity:read", Action::"identity:read:displayName"],
		resource is Identity
	);`)

	// 3. Get grace.
	asAda.Get(gracePath).AssertResource(`{"id":"` + grace.ID.String() + `","displayName":"grace"}`)

	// 4. List.
	listed := asAda.Get(identitiesPath).DecodeCollection[identity](http.StatusOK).Data
	require.Len(t, listed, 3)
	assert.Equal(t, identity{ID: listed[0].ID, DisplayName: envelope.Visible("Administrator")}, listed[0])
	assert.Equal(t, []identity{ada, {ID: grace.ID, DisplayName: grace.DisplayName}}, listed[1:])
}

// TestIdentities_ForbidOneField checks that a forbid on reading one field hides it, even where another
// policy grants it:
//
//  1. Create ada with an API key, grace, and the group auditors with ada as a member: 201 and 204.
//  2. Add a policy that lets auditors read everything but forbids them to read usernames: 201.
//  3. As ada, get grace: 200 with every field but her username.
//  4. As ada, get ada: 200 with every field but her username, though the seeded policy lets her read
//     herself, since the forbid overrides it.
func TestIdentities_ForbidOneField(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace, _ := apptest.NewIdentity(api, "grace")
	auditors := api.Post("/api/v1/groups", `{"name":"auditors"}`).
		DecodeResource[groupapi.GroupResponse](http.StatusCreated)
	api.Put("/api/v1/groups/"+auditors.ID.String()+"/members/"+ada.ID.String(), "").AssertNoContent()

	// 2. Add the policy.
	addPolicy(t, api, `permit (principal in Group::"`+auditors.ID.String()+`", action in Action::"read", resource);
		forbid (
			principal in Group::"`+auditors.ID.String()+`",
			action == Action::"identity:read:username",
			resource
		);`)

	// 3. Get grace.
	withoutUsername := grace
	withoutUsername.Username = envelope.Field[string]{}
	assert.Equal(t, withoutUsername,
		asAda.Get(identitiesPath + "/" + grace.ID.String()).DecodeResource[identity](http.StatusOK))

	// 4. Get ada.
	withoutUsername = ada
	withoutUsername.Username = envelope.Field[string]{}
	assert.Equal(t, withoutUsername,
		asAda.Get(identitiesPath + "/" + ada.ID.String()).DecodeResource[identity](http.StatusOK))
}

// TestIdentities_CreateUnreadable checks that creating an identity that the caller may not read reveals
// nothing about it but its ID:
//
//  1. Create ada with an API key: 201.
//  2. Add a policy that lets ada create identities, but not read them: 201.
//  3. As ada, create carol: 201 with her ID alone, at her Location.
//  4. As ada, get carol: 404 identity-not-found. As admin, get her: 200 with every field.
func TestIdentities_CreateUnreadable(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create ada.
	ada, asAda := apptest.NewIdentity(api, "ada")

	// 2. Add the policy.
	addPolicy(t, api, `permit (
		principal == Identity::"`+ada.ID.String()+`",
		action in Action::"identity:create",
		resource
	);`)

	// 3. Create carol.
	created := asAda.Post(identitiesPath, `{"username":"carol","displayName":"Carol"}`)
	carol := created.DecodeResource[identity](http.StatusCreated)
	carolPath := identitiesPath + "/" + carol.ID.String()
	created.AssertCreated(carolPath, `{"id":"`+carol.ID.String()+`"}`)

	// 4. Get carol.
	asAda.Get(carolPath).AssertProblemType(identityNotFound)
	got := api.Get(carolPath).DecodeResource[identity](http.StatusOK)
	assert.Equal(t, envelope.Visible("carol"), got.Username)
	assert.Equal(t, envelope.Visible(ada.ID), got.CreatedBy)
}

// TestIdentities_Patch changes ada field by field with JSON Merge Patches, as admin:
//
//  1. Create ada and grace: 201 for each.
//  2. Patch ada's display name: 200 with the new display name, her username kept and the update time
//     advanced.
//  3. Patch ada with an empty object: 200 with ada as step 2 left her, the update time included.
//  4. Patch ada's username to null: 422 invalid-body pointing to username. To a number: 400 naming
//     username. Patch her Username, in another case: 400 naming Username as not allowed, since member names
//     match exactly. Patch her username twice in one body: 400 naming username as duplicate. Patch her with
//     application/json: 415 naming application/merge-patch+json in Accept-Patch. Get her: as step 2 left her.
//  5. Patch ada's username to grace: 409 username-taken, the same as replacing it.
//  6. Patch an identity that doesn't exist: 404 identity-not-found.
func TestIdentities_Patch(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada := api.Post(identitiesPath, `{"username":"ada","displayName":"Ada"}`).
		DecodeResource[identity](http.StatusCreated)
	adaPath := identitiesPath + "/" + ada.ID.String()
	api.Post(identitiesPath, `{"username":"grace","displayName":"Grace Hopper"}`).AssertStatus(http.StatusCreated)

	// 2. Patch the display name.
	patched := api.Patch(adaPath, `{"displayName":"Ada Lovelace"}`).DecodeResource[identity](http.StatusOK)
	assert.Equal(t, envelope.Visible("Ada Lovelace"), patched.DisplayName)
	assert.Equal(t, ada.Username, patched.Username, "a member that the patch leaves out must not change")
	assert.True(t, patched.UpdatedAt.Value.After(ada.UpdatedAt.Value), "the update time must advance")

	// 3. Patch nothing.
	assert.Equal(t, patched, api.Patch(adaPath, `{}`).DecodeResource[identity](http.StatusOK))

	// 4. Patch with invalid bodies.
	resp := api.Patch(adaPath, `{"username":null}`)
	resp.AssertProblemType(invalidBody)
	assert.Contains(t, resp.Body.String(), `"pointer":"#/username"`)
	resp = api.Patch(adaPath, `{"username":5}`)
	resp.AssertProblemType(badRequest)
	assert.Contains(t, resp.Body.String(), `The member \"username\" must be of type string.`)
	resp = api.Patch(adaPath, `{"Username":"ada2"}`)
	resp.AssertProblemType(badRequest)
	assert.Contains(t, resp.Body.String(), `The member \"Username\" is not allowed.`)
	resp = api.Patch(adaPath, `{"username":"ada2","username":"ada3"}`)
	resp.AssertProblemType(badRequest)
	assert.Contains(t, resp.Body.String(), `The member \"username\" appears more than once.`)
	resp = api.Patch(adaPath, `{"Username":"ada2"}`)
	resp.AssertProblemType(badRequest)
	assert.Contains(t, resp.Body.String(), `The member \"Username\" is not allowed.`)
	resp = api.Patch(adaPath, `{"username":"ada2","username":"ada3"}`)
	resp.AssertProblemType(badRequest)
	assert.Contains(t, resp.Body.String(), `The member \"username\" appears more than once.`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, adaPath,
		strings.NewReader(`{"displayName":"Ada"}`))
	req.Header.Set("Content-Type", "application/json")
	resp = api.Do(req)
	resp.AssertProblemType(unsupportedMedia)
	assert.Equal(t, "application/merge-patch+json", resp.Header().Get("Accept-Patch"))
	assert.Equal(t, patched, api.Get(adaPath).DecodeResource[identity](http.StatusOK))

	// 5. Patch to a taken username.
	api.Patch(adaPath, `{"username":"grace"}`).AssertProblemType(usernameTaken)

	// 6. Patch a missing identity.
	api.Patch(identitiesPath+"/"+uuid.NewV4().String(), `{}`).AssertProblemType(identityNotFound)
}

// TestIdentities_PatchConcurrently checks that patches of different fields at the same time both survive,
// rather than one of them writing back the other's old value:
//
//  1. Create ada: 201.
//  2. Patch her username and, at the same time, her display name: 200 for both.
//  3. Get her: the new username and the new display name.
func TestIdentities_PatchConcurrently(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada := api.Post(identitiesPath, `{"username":"ada","displayName":"Ada"}`).
		DecodeResource[identity](http.StatusCreated)
	adaPath := identitiesPath + "/" + ada.ID.String()

	// 2. Patch both fields at once.
	bodies := []string{`{"username":"ada-lovelace"}`, `{"displayName":"Ada Lovelace"}`}
	resps := make([]*apitest.Response, len(bodies))
	var wg sync.WaitGroup
	for i, body := range bodies {
		wg.Go(func() { resps[i] = api.Patch(adaPath, body) })
	}
	wg.Wait()
	for _, resp := range resps {
		resp.AssertStatus(http.StatusOK)
	}

	// 3. Get.
	got := api.Get(adaPath).DecodeResource[identity](http.StatusOK)
	assert.Equal(t, envelope.Visible("ada-lovelace"), got.Username)
	assert.Equal(t, envelope.Visible("Ada Lovelace"), got.DisplayName)
}

// TestIdentities_PatchOneField checks that a caller who may set some fields of an identity can patch those,
// but not the others, and still can't replace it:
//
//  1. Create ada and grace, each with an API key: 201 for each.
//  2. Add a policy that lets every identity update itself, but forbids changing usernames: 201.
//  3. As ada, patch her display name: 200 with the new display name.
//  4. As ada, patch her username, and her username and display name: 403 fields-denied pointing to username
//     alone for both, and, as admin, ada is as step 3 left her, so nothing was written.
//  5. As ada, replace ada with her current username: 403 fields-denied pointing to username.
//  6. As ada, patch grace, whom she can't read, with a display name and with an empty object: the same 404
//     identity-not-found, to the byte, as for an identity that doesn't exist.
func TestIdentities_PatchOneField(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	adaPath := identitiesPath + "/" + ada.ID.String()
	grace, _ := apptest.NewIdentity(api, "grace")
	gracePath := identitiesPath + "/" + grace.ID.String()
	missingPath := identitiesPath + "/" + uuid.NewV4().String()

	// 2. Add the policy.
	addPolicy(t, api, `permit (principal, action in Action::"identity:update", resource)
			when { resource == principal };
		forbid (principal, action == Action::"identity:update:username", resource);`)

	// 3. Patch the display name.
	patched := asAda.Patch(adaPath, `{"displayName":"Ada Lovelace"}`).DecodeResource[identity](http.StatusOK)
	assert.Equal(t, envelope.Visible("Ada Lovelace"), patched.DisplayName)
	assert.Equal(t, envelope.Visible(ada.ID), patched.UpdatedBy)

	// 4. Patch the username.
	assertFieldsDenied(t, asAda.Patch(adaPath, `{"username":"ada-lovelace"}`), "#/username")
	assertFieldsDenied(t, asAda.Patch(adaPath, `{"username":"ada-lovelace","displayName":"Countess"}`), "#/username")
	assert.Equal(t, patched, api.Get(adaPath).DecodeResource[identity](http.StatusOK))

	// 5. Replace.
	assertFieldsDenied(t, asAda.Put(adaPath, `{"username":"ada","displayName":"Countess"}`), "#/username")

	// 6. Patch grace.
	for _, body := range []string{`{"displayName":"Grace"}`, `{}`} {
		hidden := asAda.Patch(gracePath, body)
		hidden.AssertProblemType(identityNotFound)
		assert.Equal(t, asAda.Patch(missingPath, body).Body.String(), hidden.Body.String(), body)
	}
}

// TestIdentities_MCPWhoami checks that an MCP client acts as the identity whose key it sends:
//
//  1. Create ada with an API key: 201.
//  2. As ada, call identity_whoami over MCP: ada, as the API created her, since the seeded policies let
//     an identity read itself.
func TestIdentities_MCPWhoami(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")

	// 2. Who am I.
	assert.Equal(t, ada, apptest.MCP(t, asAda).Call("identity_whoami", "").Decode[identity]())
}
