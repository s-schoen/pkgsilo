// End-to-end tests of the identity endpoints: each test drives the whole app through /api/v1, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps in order,
// each with the result it expects; the numbered comments in the test mark where each step starts.
package identity_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/envelope"
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
//  3. List the identities: ada alone.
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
	assert.Equal(t, []identity{ada}, apitest.DecodeCollection[identity](api.Get(identitiesPath), http.StatusOK).Data)

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
//  1. Create ada, grace and margaret, in that order: 201 for each.
//  2. List the first page of 2: ada and grace, and a nextOffset of 2.
//  3. List from offset 2: margaret alone, on the last page, without a nextOffset.
func TestIdentities_ListPages(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	var created []identity
	for _, username := range []string{"ada", "grace", "margaret"} {
		resp := api.Post(identitiesPath, `{"username":"`+username+`","displayName":"`+username+`"}`)
		created = append(created, apitest.DecodeResource[identity](resp, http.StatusCreated))
	}

	// 2. List the first page.
	first := apitest.DecodeCollection[identity](api.Get(identitiesPath+"?limit=2"), http.StatusOK)
	assert.Equal(t, created[:2], first.Data, "identities are listed in creation order")
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 2, NextOffset: new(2)}, first.Pagination)

	// 3. List the last page.
	last := apitest.DecodeCollection[identity](api.Get(identitiesPath+"?offset=2&limit=2"), http.StatusOK)
	assert.Equal(t, created[2:], last.Data)
	assert.Equal(t, envelope.Pagination{Offset: 2, Limit: 2}, last.Pagination)
}
