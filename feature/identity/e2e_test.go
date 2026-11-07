// End-to-end tests of the identity endpoints: each test drives the whole app through /api/v1, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps in order,
// each with the result it expects; the numbered comments in the test mark where each step starts.
package identity_test

import (
	"net/http"
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
	ada := apitest.DecodeResource[identity](created, http.StatusCreated)
	location := identitiesPath + "/" + ada.ID.String()
	assert.Equal(t, location, created.Header().Get("Location"))
	assert.Equal(t, "ada", ada.Username)
	assert.Equal(t, "Ada Lovelace", ada.DisplayName)
	assert.Equal(t, ada.CreatedBy, ada.UpdatedBy)
	assert.WithinDuration(t, time.Now(), ada.CreatedAt, time.Minute)
	assert.Equal(t, ada.CreatedAt, ada.UpdatedAt)

	// 2. Get.
	assert.Equal(t, ada, apitest.DecodeResource[identity](api.Get(location), http.StatusOK))

	// 3. List.
	listed := apitest.DecodeCollection[identity](api.Get(identitiesPath), http.StatusOK).Data
	require.Len(t, listed, 2)
	assert.Equal(t, "admin", listed[0].Username)
	assert.Equal(t, ada, listed[1])
	assert.Equal(t, listed[0].ID, ada.CreatedBy)

	// 4. Replace.
	replaced := apitest.DecodeResource[identity](
		api.Put(location, `{"username":"ada-lovelace","displayName":"Countess of Lovelace"}`), http.StatusOK)
	assert.Equal(t, ada.ID, replaced.ID)
	assert.Equal(t, "ada-lovelace", replaced.Username)
	assert.Equal(t, "Countess of Lovelace", replaced.DisplayName)
	assert.Equal(t, ada.CreatedAt, replaced.CreatedAt, "the creation time is immutable")
	assert.True(t, replaced.UpdatedAt.After(ada.UpdatedAt), "the update time must advance")

	// 5. Get again.
	assert.Equal(t, replaced, apitest.DecodeResource[identity](api.Get(location), http.StatusOK))

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
	grace := apitest.DecodeResource[identity](
		api.Post(identitiesPath, `{"username":"grace","displayName":"Grace Hopper"}`), http.StatusCreated)
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
		assert.Equal(t, grace, apitest.DecodeResource[identity](api.Get(gracePath), http.StatusOK))
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
		created = append(created, apitest.DecodeResource[identity](resp, http.StatusCreated))
	}

	// 2. List the first page.
	first := apitest.DecodeCollection[identity](api.Get(identitiesPath+"?limit=2"), http.StatusOK)
	require.Len(t, first.Data, 2)
	assert.Equal(t, "admin", first.Data[0].Username, "identities are listed in creation order")
	assert.Equal(t, created[0], first.Data[1])
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 2, NextOffset: new(2)}, first.Pagination)

	// 3. List the last page.
	last := apitest.DecodeCollection[identity](api.Get(identitiesPath+"?offset=2&limit=2"), http.StatusOK)
	assert.Equal(t, created[1:], last.Data)
	assert.Equal(t, envelope.Pagination{Offset: 2, Limit: 2}, last.Pagination)
}

// addPolicy stores a policy and announces it, like every policy write does. The policies can't be managed
// through the API yet, so they're written directly.
func addPolicy(t *testing.T, db *sqlx.DB, source string) {
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
	assert.Equal(t, ada, apitest.DecodeResource[identity](asAda.Get(identitiesPath+"/"+ada.ID.String()), http.StatusOK))

	// 3. Get grace.
	hidden := asAda.Get(gracePath)
	hidden.AssertProblemType(identityNotFound)
	missing := asAda.Get(identitiesPath + "/" + uuid.NewV4().String())
	assert.Equal(t, missing.Body.String(), hidden.Body.String(), "a hidden identity must look like a missing one")

	// 4. List.
	listed := apitest.DecodeCollection[identity](asAda.Get(identitiesPath), http.StatusOK)
	assert.Equal(t, []identity{ada}, listed.Data)
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 50}, listed.Pagination)

	// 5. List from offset 1.
	past := apitest.DecodeCollection[identity](asAda.Get(identitiesPath+"?offset=1"), http.StatusOK)
	assert.Empty(t, past.Data)
	assert.Equal(t, envelope.Pagination{Offset: 1, Limit: 50}, past.Pagination)

	// 6. As admin.
	assert.Equal(t, grace, apitest.DecodeResource[identity](api.Get(gracePath), http.StatusOK))
	all := apitest.DecodeCollection[identity](api.Get(identitiesPath), http.StatusOK).Data
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
	api, db := apptest.NewWithDB(t)

	// 1. Create.
	reader, asReader := apptest.NewIdentity(api, "reader")
	created := map[string]identity{}
	for _, username := range []string{"one", "two", "three", "four", "five", "six"} {
		resp := api.Post(identitiesPath, `{"username":"`+username+`","displayName":"`+username+`"}`)
		created[username] = apitest.DecodeResource[identity](resp, http.StatusCreated)
	}
	group := apitest.DecodeResource[groupapi.GroupResponse](
		api.Post("/api/v1/groups", `{"name":"visible"}`), http.StatusCreated)
	for _, username := range []string{"two", "four", "six"} {
		api.Put("/api/v1/groups/"+group.ID.String()+"/members/"+created[username].ID.String(), "").AssertNoContent()
	}

	// 2. List before the policy.
	assert.Equal(t, []identity{reader},
		apitest.DecodeCollection[identity](asReader.Get(identitiesPath), http.StatusOK).Data)

	// 3. Add the policy.
	addPolicy(t, db, `permit (
		principal == Identity::"`+reader.ID.String()+`",
		action in Action::"read",
		resource in Group::"`+group.ID.String()+`"
	);`)

	// 4. List the first page.
	first := apitest.DecodeCollection[identity](asReader.Get(identitiesPath+"?limit=2"), http.StatusOK)
	assert.Equal(t, []identity{reader, created["two"]}, first.Data)
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 2, NextOffset: new(2)}, first.Pagination)

	// 5. List the last page.
	last := apitest.DecodeCollection[identity](asReader.Get(identitiesPath+"?offset=2&limit=2"), http.StatusOK)
	assert.Equal(t, []identity{created["four"], created["six"]}, last.Data)
	assert.Equal(t, envelope.Pagination{Offset: 2, Limit: 2}, last.Pagination)

	// 6. Get one by one.
	asReader.Get(identitiesPath + "/" + created["three"].ID.String()).AssertProblemType(identityNotFound)
	assert.Equal(t, created["four"],
		apitest.DecodeResource[identity](asReader.Get(identitiesPath+"/"+created["four"].ID.String()), http.StatusOK))
}

// TestIdentities_FailingPolicy checks that a policy that fails to evaluate results in an error rather than
// in access, even for the admin, whom another policy allows everything:
//
//  1. Add a forbid that fails to evaluate on every request.
//  2. Get the admin and list the identities: 500 for both, without details.
func TestIdentities_FailingPolicy(t *testing.T) {
	t.Parallel()
	api, db := apptest.NewWithDB(t)
	admin := apitest.DecodeCollection[identity](api.Get(identitiesPath), http.StatusOK).Data[0]

	// 1. Add the policy.
	addPolicy(t, db, `forbid (principal, action, resource) when { resource.missing == principal };`)

	// 2. Get and list.
	api.Get(identitiesPath + "/" + admin.ID.String()).AssertInternalError()
	api.Get(identitiesPath).AssertInternalError()
}
