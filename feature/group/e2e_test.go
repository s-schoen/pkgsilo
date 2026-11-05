// End-to-end tests of the group endpoints: each test drives the whole app through /api/v1, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps in order,
// each with the result it expects; the numbered comments in the test mark where each step starts.
package group_test

import (
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/envelope"
	groupapi "pkgsilo/feature/group/api"
	identityapi "pkgsilo/feature/identity/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/apptest"
)

// The problem types of the group endpoints, written out independently of the code: clients tell
// problems apart by them, so a changed slug, title or status must fail these tests.
var (
	groupNotFound    = envelope.DefineProblemType("group-not-found", "Group not found", http.StatusNotFound)
	groupNameTaken   = envelope.DefineProblemType("group-name-taken", "Group name taken", http.StatusConflict)
	identityNotFound = envelope.DefineProblemType("identity-not-found", "Identity not found", http.StatusNotFound)
	memberNotFound   = envelope.DefineProblemType("member-not-found", "Member not found", http.StatusNotFound)
)

const (
	groupsPath     = "/api/v1/groups"
	identitiesPath = "/api/v1/identities"
)

type (
	group    = groupapi.GroupResponse
	member   = groupapi.MemberResponse
	identity = identityapi.IdentityResponse
)

// createGroup creates a group with the given name and returns it with its path.
func createGroup(api *apitest.Client, name string) (created group, path string) {
	created = apitest.DecodeResource[group](api.Post(groupsPath, `{"name":"`+name+`"}`), http.StatusCreated)

	return created, groupsPath + "/" + created.ID.String()
}

// createIdentity creates an identity with the given username and returns its ID.
func createIdentity(api *apitest.Client, username string) uuid.UUID {
	resp := api.Post(identitiesPath, `{"username":"`+username+`","displayName":"`+username+`"}`)

	return apitest.DecodeResource[identity](resp, http.StatusCreated).ID
}

// adminID returns the ID of the identity that the client of apptest.New authenticates as: the one that
// the first start created, before all others.
func adminID(api *apitest.Client) uuid.UUID {
	return apitest.DecodeCollection[identity](api.Get(identitiesPath+"?limit=1"), http.StatusOK).Data[0].ID
}

// memberIDs lists the members of the group at groupPath and returns their identity IDs, in order.
func memberIDs(api *apitest.Client, groupPath string) []uuid.UUID {
	members := apitest.DecodeCollection[member](api.Get(groupPath+"/members"), http.StatusOK).Data
	ids := make([]uuid.UUID, len(members))
	for i, m := range members {
		ids[i] = m.IdentityID
	}

	return ids
}

// TestGroups_Lifecycle takes one group, maintainers, through every group endpoint:
//
//  1. Create maintainers: 201 with the new group, its Location under /api/v1/groups, and its
//     audit fields set by the server.
//  2. Get it through the Location: the created group.
//  3. List the groups: maintainers alone.
//  4. Replace its name: 200 with the new name, the ID and creation time kept and the update time
//     advanced.
//  5. Get it again: the replaced group, so the change was stored.
//  6. Delete it: 204.
//  7. Get and delete it again: 404 group-not-found for both.
func TestGroups_Lifecycle(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	created := api.Post(groupsPath, `{"name":"Maintainers"}`)
	maintainers := apitest.DecodeResource[group](created, http.StatusCreated)
	location := groupsPath + "/" + maintainers.ID.String()
	assert.Equal(t, location, created.Header().Get("Location"))
	assert.Equal(t, "Maintainers", maintainers.Name)
	assert.Equal(t, maintainers.CreatedBy, maintainers.UpdatedBy)
	assert.WithinDuration(t, time.Now(), maintainers.CreatedAt, time.Minute)
	assert.Equal(t, maintainers.CreatedAt, maintainers.UpdatedAt)

	// 2. Get.
	assert.Equal(t, maintainers, apitest.DecodeResource[group](api.Get(location), http.StatusOK))

	// 3. List.
	assert.Equal(t, []group{maintainers}, apitest.DecodeCollection[group](api.Get(groupsPath), http.StatusOK).Data)

	// 4. Replace.
	replaced := apitest.DecodeResource[group](api.Put(location, `{"name":"Core maintainers"}`), http.StatusOK)
	assert.Equal(t, maintainers.ID, replaced.ID)
	assert.Equal(t, "Core maintainers", replaced.Name)
	assert.Equal(t, maintainers.CreatedAt, replaced.CreatedAt, "the creation time is immutable")
	assert.True(t, replaced.UpdatedAt.After(maintainers.UpdatedAt), "the update time must advance")

	// 5. Get again.
	assert.Equal(t, replaced, apitest.DecodeResource[group](api.Get(location), http.StatusOK))

	// 6. Delete.
	api.Delete(location).AssertNoContent()

	// 7. Get and delete again.
	api.Get(location).AssertProblemType(groupNotFound)
	api.Delete(location).AssertProblemType(groupNotFound)
}

// TestGroups_NameTaken checks that the database's unique name constraint reaches clients as a
// conflict:
//
//  1. Create maintainers and reviewers: 201 for both.
//  2. Create another group named Maintainers: 409 group-name-taken.
//  3. Rename reviewers to Maintainers: 409 group-name-taken, and reviewers is unchanged.
//
// Steps 2 and 3 are parallel subtests on the same app.
func TestGroups_NameTaken(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create maintainers and reviewers.
	createGroup(api, "Maintainers")
	reviewers, reviewersPath := createGroup(api, "Reviewers")

	// 2. Create another Maintainers.
	t.Run("create", func(t *testing.T) {
		t.Parallel()
		api := api.For(t)

		api.Post(groupsPath, `{"name":"Maintainers"}`).AssertProblemType(groupNameTaken)
	})
	// 3. Rename reviewers to Maintainers.
	t.Run("replace", func(t *testing.T) {
		t.Parallel()
		api := api.For(t)

		api.Put(reviewersPath, `{"name":"Maintainers"}`).AssertProblemType(groupNameTaken)
		assert.Equal(t, reviewers, apitest.DecodeResource[group](api.Get(reviewersPath), http.StatusOK))
	})
}

// TestGroups_ListPages pages through the groups, which are listed in creation order:
//
//  1. Create maintainers, reviewers and releasers, in that order: 201 for each.
//  2. List the first page of 2: maintainers and reviewers, and a nextOffset of 2.
//  3. List from offset 2: releasers alone, on the last page, without a nextOffset.
func TestGroups_ListPages(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	var created []group
	for _, name := range []string{"Maintainers", "Reviewers", "Releasers"} {
		g, _ := createGroup(api, name)
		created = append(created, g)
	}

	// 2. List the first page.
	first := apitest.DecodeCollection[group](api.Get(groupsPath+"?limit=2"), http.StatusOK)
	assert.Equal(t, created[:2], first.Data, "groups are listed in creation order")
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 2, NextOffset: new(2)}, first.Pagination)

	// 3. List the last page.
	last := apitest.DecodeCollection[group](api.Get(groupsPath+"?offset=2&limit=2"), http.StatusOK)
	assert.Equal(t, created[2:], last.Data)
	assert.Equal(t, envelope.Pagination{Offset: 2, Limit: 2}, last.Pagination)
}

// TestGroups_Members takes the members of one group through every member endpoint:
//
//  1. Create the identities ada, grace and margaret and the group maintainers: 201 for each.
//  2. List the members of maintainers: none.
//  3. Add ada: 204.
//  4. Add ada again: 204, and her membership keeps its creation time.
//  5. Add grace and margaret: 204 for both.
//  6. List the first page of 2 members: ada and grace in the order they were added, with ada's
//     membership attributed and timed by the server, and a nextOffset of 2.
//  7. List from offset 2: margaret alone, on the last page.
//  8. Remove grace: 204, and ada and margaret remain.
//  9. Remove grace again: 404 member-not-found.
func TestGroups_Members(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada := createIdentity(api, "ada")
	grace := createIdentity(api, "grace")
	margaret := createIdentity(api, "margaret")
	_, maintainersPath := createGroup(api, "Maintainers")
	membersPath := maintainersPath + "/members"

	// 2. List none.
	assert.Empty(t, memberIDs(api, maintainersPath))

	// 3. Add ada.
	api.Put(membersPath+"/"+ada.String(), "").AssertNoContent()
	added := apitest.DecodeCollection[member](api.Get(membersPath), http.StatusOK).Data

	// 4. Add ada again.
	api.Put(membersPath+"/"+ada.String(), "").AssertNoContent()
	assert.Equal(t, added, apitest.DecodeCollection[member](api.Get(membersPath), http.StatusOK).Data,
		"adding a member again must not change the membership")

	// 5. Add grace and margaret.
	api.Put(membersPath+"/"+grace.String(), "").AssertNoContent()
	api.Put(membersPath+"/"+margaret.String(), "").AssertNoContent()

	// 6. List the first page.
	first := apitest.DecodeCollection[member](api.Get(membersPath+"?limit=2"), http.StatusOK)
	if assert.Len(t, first.Data, 2) {
		assert.Equal(t, ada, first.Data[0].IdentityID, "members are listed in the order they were added")
		assert.Equal(t, grace, first.Data[1].IdentityID)
		assert.Equal(t, adminID(api), first.Data[0].CreatedBy, "the change is attributed to the caller")
		assert.WithinDuration(t, time.Now(), first.Data[0].CreatedAt, time.Minute)
	}
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 2, NextOffset: new(2)}, first.Pagination)

	// 7. List the last page.
	last := apitest.DecodeCollection[member](api.Get(membersPath+"?offset=2&limit=2"), http.StatusOK)
	if assert.Len(t, last.Data, 1) {
		assert.Equal(t, margaret, last.Data[0].IdentityID)
	}
	assert.Equal(t, envelope.Pagination{Offset: 2, Limit: 2}, last.Pagination)

	// 8. Remove grace.
	api.Delete(membersPath + "/" + grace.String()).AssertNoContent()
	assert.Equal(t, []uuid.UUID{ada, margaret}, memberIDs(api, maintainersPath))

	// 9. Remove grace again.
	api.Delete(membersPath + "/" + grace.String()).AssertProblemType(memberNotFound)
}

// TestGroups_MemberReferences checks that the database's foreign keys between groups, members and
// identities reach clients:
//
//  1. Create the identities ada and grace and the groups maintainers and reviewers, and add ada
//     and grace to both groups: 201 and 204 for each.
//  2. Add an identity that doesn't exist, and one whose ID isn't a UUID, to maintainers: 404
//     identity-not-found for both.
//  3. List members of a group that doesn't exist, and add and remove ada, an identity that
//     doesn't exist and one whose ID isn't a UUID: 404 group-not-found for each, so a missing
//     group is reported before a missing identity.
//  4. Delete grace: 204, and she is no longer a member of maintainers.
//  5. Delete reviewers, which still has members: 204.
//  6. Delete ada: 204, and maintainers has no members left.
func TestGroups_MemberReferences(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create and add.
	ada := createIdentity(api, "ada")
	grace := createIdentity(api, "grace")
	_, maintainersPath := createGroup(api, "Maintainers")
	_, reviewersPath := createGroup(api, "Reviewers")
	for _, groupPath := range []string{maintainersPath, reviewersPath} {
		for _, id := range []uuid.UUID{ada, grace} {
			api.Put(groupPath+"/members/"+id.String(), "").AssertNoContent()
		}
	}

	// 2. Add a missing identity.
	missingIdentityIDs := []string{uuid.NewV4().String(), "not-a-uuid"}
	for _, id := range missingIdentityIDs {
		api.Put(maintainersPath+"/members/"+id, "").AssertProblemType(identityNotFound)
	}

	// 3. Use a missing group.
	missingMembersPath := groupsPath + "/" + uuid.NewV4().String() + "/members"
	api.Get(missingMembersPath).AssertProblemType(groupNotFound)
	for _, id := range append([]string{ada.String()}, missingIdentityIDs...) {
		api.Put(missingMembersPath+"/"+id, "").AssertProblemType(groupNotFound)
		api.Delete(missingMembersPath + "/" + id).AssertProblemType(groupNotFound)
	}

	// 4. Delete grace.
	api.Delete(identitiesPath + "/" + grace.String()).AssertNoContent()
	assert.Equal(t, []uuid.UUID{ada}, memberIDs(api, maintainersPath), "a deleted identity leaves its groups")

	// 5. Delete reviewers.
	api.Delete(reviewersPath).AssertNoContent()

	// 6. Delete ada.
	api.Delete(identitiesPath + "/" + ada.String()).AssertNoContent()
	assert.Empty(t, memberIDs(api, maintainersPath))
}
