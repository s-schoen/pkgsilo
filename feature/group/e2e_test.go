// End-to-end tests of the group endpoints: each test drives the whole app through /api/v1, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps in order,
// each with the result it expects; the numbered comments in the test mark where each step starts.
package group_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	fieldsDenied     = envelope.DefineProblemType("fields-denied", "Fields denied", http.StatusForbidden)
	invalidBody      = envelope.DefineProblemType("invalid-body", "Invalid request body", http.StatusUnprocessableEntity)
	forbidden        = envelope.ProblemType{Type: "about:blank", Title: "Forbidden", Status: http.StatusForbidden}
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
	created = api.Post(groupsPath, `{"name":"`+name+`"}`).DecodeResource[group](http.StatusCreated)

	return created, groupsPath + "/" + created.ID.String()
}

// createIdentity creates an identity with the given username and returns its ID.
func createIdentity(api *apitest.Client, username string) uuid.UUID {
	resp := api.Post(identitiesPath, `{"username":"`+username+`","displayName":"`+username+`"}`)

	return resp.DecodeResource[identity](http.StatusCreated).ID
}

// adminID returns the ID of the identity that the client of apptest.New authenticates as: the one that
// the first start created, before all others.
func adminID(api *apitest.Client) uuid.UUID {
	return api.Get(identitiesPath + "?limit=1").DecodeCollection[identity](http.StatusOK).Data[0].ID
}

// memberIDs lists the members of the group at groupPath and returns their identity IDs, in order.
func memberIDs(api *apitest.Client, groupPath string) []uuid.UUID {
	members := api.Get(groupPath + "/members").DecodeCollection[member](http.StatusOK).Data
	ids := make([]uuid.UUID, len(members))
	for i, m := range members {
		ids[i] = m.IdentityID.Value
	}

	return ids
}

// TestGroups_Lifecycle takes one group, maintainers, through every group endpoint:
//
//  1. Create maintainers: 201 with the new group, its Location under /api/v1/groups, and its
//     audit fields set by the server.
//  2. Get it through the Location: the created group.
//  3. List the groups: administrators, which the first start created, then maintainers.
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
	maintainers := created.DecodeResource[group](http.StatusCreated)
	location := groupsPath + "/" + maintainers.ID.String()
	assert.Equal(t, location, created.Header().Get("Location"))
	assert.Equal(t, envelope.Visible("Maintainers"), maintainers.Name)
	assert.Equal(t, maintainers.CreatedBy, maintainers.UpdatedBy)
	assert.WithinDuration(t, time.Now(), maintainers.CreatedAt.Value, time.Minute)
	assert.Equal(t, maintainers.CreatedAt, maintainers.UpdatedAt)

	// 2. Get.
	assert.Equal(t, maintainers, api.Get(location).DecodeResource[group](http.StatusOK))

	// 3. List.
	listed := api.Get(groupsPath).DecodeCollection[group](http.StatusOK).Data
	if assert.Len(t, listed, 2) {
		assert.Equal(t, envelope.Visible("administrators"), listed[0].Name)
		assert.Equal(t, maintainers, listed[1])
	}

	// 4. Replace.
	replaced := api.Put(location, `{"name":"Core maintainers"}`).DecodeResource[group](http.StatusOK)
	assert.Equal(t, maintainers.ID, replaced.ID)
	assert.Equal(t, envelope.Visible("Core maintainers"), replaced.Name)
	assert.Equal(t, maintainers.CreatedAt, replaced.CreatedAt, "the creation time is immutable")
	assert.True(t, replaced.UpdatedAt.Value.After(maintainers.UpdatedAt.Value), "the update time must advance")

	// 5. Get again.
	assert.Equal(t, replaced, api.Get(location).DecodeResource[group](http.StatusOK))

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
		assert.Equal(t, reviewers, api.Get(reviewersPath).DecodeResource[group](http.StatusOK))
	})
}

// TestGroups_ListPages pages through the groups, which are listed in creation order:
//
//  1. Create maintainers, reviewers and releasers, in that order: 201 for each.
//  2. List the first page of 2: administrators, which the first start created before them, then maintainers,
//     and a nextOffset of 2.
//  3. List from offset 2: reviewers and releasers, on the last page, without a nextOffset.
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
	first := api.Get(groupsPath + "?limit=2").DecodeCollection[group](http.StatusOK)
	if assert.Len(t, first.Data, 2) {
		assert.Equal(t, envelope.Visible("administrators"), first.Data[0].Name, "groups are listed in creation order")
		assert.Equal(t, created[0], first.Data[1])
	}
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 2, NextOffset: new(2)}, first.Pagination)

	// 3. List the last page.
	last := api.Get(groupsPath + "?offset=2&limit=2").DecodeCollection[group](http.StatusOK)
	assert.Equal(t, created[1:], last.Data)
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
	added := api.Get(membersPath).DecodeCollection[member](http.StatusOK).Data

	// 4. Add ada again.
	api.Put(membersPath+"/"+ada.String(), "").AssertNoContent()
	assert.Equal(t, added, api.Get(membersPath).DecodeCollection[member](http.StatusOK).Data,
		"adding a member again must not change the membership")

	// 5. Add grace and margaret.
	api.Put(membersPath+"/"+grace.String(), "").AssertNoContent()
	api.Put(membersPath+"/"+margaret.String(), "").AssertNoContent()

	// 6. List the first page.
	first := api.Get(membersPath + "?limit=2").DecodeCollection[member](http.StatusOK)
	if assert.Len(t, first.Data, 2) {
		assert.Equal(t, ada, first.Data[0].IdentityID.Value, "members are listed in the order they were added")
		assert.Equal(t, grace, first.Data[1].IdentityID.Value)
		assert.Equal(t, adminID(api), first.Data[0].CreatedBy.Value, "the change is attributed to the caller")
		assert.WithinDuration(t, time.Now(), first.Data[0].CreatedAt.Value, time.Minute)
	}
	assert.Equal(t, envelope.Pagination{Offset: 0, Limit: 2, NextOffset: new(2)}, first.Pagination)

	// 7. List the last page.
	last := api.Get(membersPath + "?offset=2&limit=2").DecodeCollection[member](http.StatusOK)
	if assert.Len(t, last.Data, 1) {
		assert.Equal(t, margaret, last.Data[0].IdentityID.Value)
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

// addMembers adds the identities to the group at groupPath through the client.
func addMembers(api *apitest.Client, groupPath string, ids ...uuid.UUID) {
	for _, id := range ids {
		api.Put(groupPath+"/members/"+id.String(), "").AssertNoContent()
	}
}

// addPolicy creates a policy with the given source, under a name of its own, through the client.
func addPolicy(t *testing.T, api *apitest.Client, source string) {
	t.Helper()

	body, err := json.Marshal(map[string]string{"name": uuid.NewV4().String(), "source": source})
	require.NoError(t, err)
	api.Post("/api/v1/policies", string(body)).AssertStatus(http.StatusCreated)
}

// listGroups returns the groups that the client may read.
func listGroups(api *apitest.Client) []group {
	return api.Get(groupsPath).DecodeCollection[group](http.StatusOK).Data
}

// listMembers returns the members of the group at groupPath, which the client may read.
func listMembers(api *apitest.Client, groupPath string) []member {
	return api.Get(groupPath + "/members").DecodeCollection[member](http.StatusOK).Data
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

// TestGroups_HiddenByDefault checks what the seeded policies give an identity outside the administrators
// group on groups, which is nothing:
//
//  1. Create ada with an API key, grace, and maintainers with grace as a member: 201 and 204.
//  2. As ada, list the groups: an empty 200.
//  3. As ada, get, replace and delete maintainers, list its members, add ada and an ID that isn't a UUID,
//     and remove grace: the same 404 group-not-found, to the byte, as for a group that doesn't exist.
//  4. As ada, create a group: 403 fields-denied pointing to name.
//  5. As admin, list the groups and maintainers' members: nothing was written.
func TestGroups_HiddenByDefault(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace := createIdentity(api, "grace")
	_, maintainersPath := createGroup(api, "Maintainers")
	addMembers(api, maintainersPath, grace)
	groups := listGroups(api)
	missingPath := groupsPath + "/" + uuid.NewV4().String()

	// 2. List.
	assert.Empty(t, listGroups(asAda))

	// 3. Use maintainers.
	for name, send := range map[string]func(api *apitest.Client, groupPath string) *apitest.Response{
		"get": (*apitest.Client).Get,
		"replace": func(api *apitest.Client, groupPath string) *apitest.Response {
			return api.Put(groupPath, `{"name":"Mine"}`)
		},
		"delete": (*apitest.Client).Delete,
		"list members": func(api *apitest.Client, groupPath string) *apitest.Response {
			return api.Get(groupPath + "/members")
		},
		"add member": func(api *apitest.Client, groupPath string) *apitest.Response {
			return api.Put(groupPath+"/members/"+ada.ID.String(), "")
		},
		"add non-UUID member": func(api *apitest.Client, groupPath string) *apitest.Response {
			return api.Put(groupPath+"/members/not-a-uuid", "")
		},
		"remove member": func(api *apitest.Client, groupPath string) *apitest.Response {
			return api.Delete(groupPath + "/members/" + grace.String())
		},
	} {
		hidden := send(asAda, maintainersPath)
		hidden.AssertProblemType(groupNotFound)
		assert.Equal(t, send(asAda, missingPath).Body.String(), hidden.Body.String(), name)
	}

	// 4. Create.
	assertFieldsDenied(t, asAda.Post(groupsPath, `{"name":"Mine"}`), "#/name")

	// 5. Nothing written.
	assert.Equal(t, groups, listGroups(api))
	assert.Equal(t, []uuid.UUID{grace}, memberIDs(api, maintainersPath))
}

// TestGroups_IdentitiesButNotGroups checks a policy that limits access by object type: the members of a group
// manage identities, but not groups.
//
//  1. Create ada with an API key, and the group identity-managers with her as a member: 201 and 204.
//  2. Add a policy that lets identity-managers read, create, replace and delete identities: 201.
//  3. As ada, create carol: 201.
//  4. As ada, list the groups: an empty 200. Get identity-managers and add carol to it: 404
//     group-not-found for both. Create a group: 403 fields-denied pointing to name.
func TestGroups_IdentitiesButNotGroups(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	managers, managersPath := createGroup(api, "identity-managers")
	addMembers(api, managersPath, ada.ID)

	// 2. Add the policy.
	addPolicy(t, api, `permit (
		principal in Group::"`+managers.ID.String()+`",
		action in [Action::"read", Action::"identity:create", Action::"identity:update", Action::"identity:delete"],
		resource is Identity
	);`)

	// 3. Manage identities.
	carol := asAda.Post(identitiesPath, `{"username":"carol","displayName":"Carol"}`).
		DecodeResource[identity](http.StatusCreated)

	// 4. Groups.
	assert.Empty(t, listGroups(asAda))
	asAda.Get(managersPath).AssertProblemType(groupNotFound)
	asAda.Put(managersPath+"/members/"+carol.ID.String(), "").AssertProblemType(groupNotFound)
	assertFieldsDenied(t, asAda.Post(groupsPath, `{"name":"Mine"}`), "#/name")
}

// TestGroups_ReadOnlyAudit checks a policy that limits access by method: the members of an audit group read
// all groups and their members, and write nothing.
//
//  1. Create ada with an API key and grace, the group auditors with ada as a member, and maintainers with
//     grace as a member: 201 and 204.
//  2. Add a policy that lets auditors read everything: 201.
//  3. As ada, list the groups and maintainers' members: the same as admin, with every field. Get maintainers:
//     200 with every field.
//  4. As ada, create a group and replace maintainers: 403 fields-denied pointing to name for both. Delete
//     maintainers, add ada to it and remove grace from it: 403 Forbidden for each.
//  5. As admin, list the groups and maintainers' members: nothing was written.
func TestGroups_ReadOnlyAudit(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace := createIdentity(api, "grace")
	auditors, auditorsPath := createGroup(api, "auditors")
	addMembers(api, auditorsPath, ada.ID)
	maintainers, maintainersPath := createGroup(api, "maintainers")
	addMembers(api, maintainersPath, grace)

	// 2. Add the policy.
	addPolicy(t, api, `permit (principal in Group::"`+auditors.ID.String()+`", action in Action::"read", resource);`)

	// 3. Read.
	groups := listGroups(api)
	members := listMembers(api, maintainersPath)
	assert.Equal(t, groups, listGroups(asAda))
	assert.Equal(t, members, listMembers(asAda, maintainersPath))
	assert.Equal(t, maintainers, asAda.Get(maintainersPath).DecodeResource[group](http.StatusOK))

	// 4. Write.
	assertFieldsDenied(t, asAda.Post(groupsPath, `{"name":"more"}`), "#/name")
	assertFieldsDenied(t, asAda.Put(maintainersPath, `{"name":"maintainers"}`), "#/name")
	asAda.Delete(maintainersPath).AssertProblemType(forbidden)
	asAda.Put(maintainersPath+"/members/"+ada.ID.String(), "").AssertProblemType(forbidden)
	asAda.Delete(maintainersPath + "/members/" + grace.String()).AssertProblemType(forbidden)

	// 5. Nothing written.
	assert.Equal(t, groups, listGroups(api))
	assert.Equal(t, members, listMembers(api, maintainersPath))
}

// TestGroups_MembersReadOwnGroup checks a policy that lets the members of each group read it and its
// members, through `principal in resource`:
//
//  1. Create ada with an API key and grace, maintainers with both as members, and reviewers with grace as
//     a member: 201 and 204.
//  2. Add a policy that lets every identity read the groups it's a member of, and their members: 201.
//  3. As ada, list the groups: maintainers alone. Get maintainers: 200. List its members: ada and grace,
//     though ada can't read grace, since reading the members covers all of them.
//  4. As ada, get reviewers and list its members: the same 404 group-not-found, to the byte, as for a group
//     that doesn't exist.
//  5. As admin, remove ada from maintainers: 204. As ada, list the groups: an empty 200. Get maintainers:
//     404 group-not-found.
func TestGroups_MembersReadOwnGroup(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace := createIdentity(api, "grace")
	maintainers, maintainersPath := createGroup(api, "maintainers")
	addMembers(api, maintainersPath, ada.ID, grace)
	_, reviewersPath := createGroup(api, "reviewers")
	addMembers(api, reviewersPath, grace)
	missingPath := groupsPath + "/" + uuid.NewV4().String()

	// 2. Add the policy.
	addPolicy(t, api, `permit (
		principal,
		action in [Action::"group:read", Action::"group:readFields", Action::"group:readMembers"],
		resource is Group
	) when { principal in resource };`)

	// 3. Read maintainers.
	assert.Equal(t, []group{maintainers}, listGroups(asAda))
	assert.Equal(t, maintainers, asAda.Get(maintainersPath).DecodeResource[group](http.StatusOK))
	assert.Equal(t, []uuid.UUID{ada.ID, grace}, memberIDs(asAda, maintainersPath))
	asAda.Get(identitiesPath + "/" + grace.String()).AssertProblemType(identityNotFound)

	// 4. Read reviewers.
	assertLikeMissing(t, asAda.Get, reviewersPath, missingPath, groupNotFound)
	assertLikeMissing(t, asAda.Get, reviewersPath+"/members", missingPath+"/members", groupNotFound)

	// 5. Leave maintainers.
	api.Delete(maintainersPath + "/members/" + ada.ID.String()).AssertNoContent()
	assert.Empty(t, listGroups(asAda))
	asAda.Get(maintainersPath).AssertProblemType(groupNotFound)
}

// TestGroups_AddHiddenIdentity checks that adding a member can't reveal whether an identity that the caller
// can't read exists, and that the identity's groups decide whether it can:
//
//  1. Create ada with an API key and grace, and the groups maintainers and visible, with grace as a member of
//     visible: 201 and 204.
//  2. Add a policy that lets ada read maintainers, read its members and add members to it: 201.
//  3. As ada, add grace to maintainers: the same 404 identity-not-found, to the byte, as for an identity that
//     doesn't exist. Maintainers has no members.
//  4. As ada, add ada, whom the seeded policies let her read: 204.
//  5. Add a policy that lets ada read the members of visible: 201. As ada, add grace: 204, and maintainers
//     has ada and grace as members.
//  6. As ada, remove grace, and an ID that isn't a UUID: 403 Forbidden for both, since no policy lets her
//     remove members.
func TestGroups_AddHiddenIdentity(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace := createIdentity(api, "grace")
	maintainers, maintainersPath := createGroup(api, "maintainers")
	visible, visiblePath := createGroup(api, "visible")
	addMembers(api, visiblePath, grace)

	// 2. Add the policy.
	addPolicy(t, api, `permit (
		principal == Identity::"`+ada.ID.String()+`",
		action in [Action::"group:read", Action::"group:readMembers", Action::"group:addMember"],
		resource == Group::"`+maintainers.ID.String()+`"
	);`)

	// 3. Add grace.
	assertLikeMissing(t, func(target string) *apitest.Response { return asAda.Put(target, "") },
		maintainersPath+"/members/"+grace.String(), maintainersPath+"/members/"+uuid.NewV4().String(),
		identityNotFound)
	assert.Empty(t, memberIDs(asAda, maintainersPath))

	// 4. Add ada.
	addMembers(asAda, maintainersPath, ada.ID)

	// 5. Add grace, readable.
	addPolicy(t, api, `permit (
		principal == Identity::"`+ada.ID.String()+`",
		action == Action::"identity:read",
		resource in Group::"`+visible.ID.String()+`"
	);`)
	addMembers(asAda, maintainersPath, grace)
	assert.Equal(t, []uuid.UUID{ada.ID, grace}, memberIDs(asAda, maintainersPath))

	// 6. Remove grace.
	asAda.Delete(maintainersPath + "/members/" + grace.String()).AssertProblemType(forbidden)
	asAda.Delete(maintainersPath + "/members/not-a-uuid").AssertProblemType(forbidden)
}

// TestGroups_RemoveHiddenMember checks that removing a member can't reveal whether an identity that the caller
// can't read exists, or is a member:
//
//  1. Create ada with an API key, grace, and maintainers with grace as a member: 201 and 204.
//  2. Add a policy that lets ada read maintainers and remove its members, but not read them: 201.
//  3. As ada, remove grace from maintainers: the same 404 member-not-found, to the byte, as for an identity
//     that doesn't exist. As admin, maintainers still has grace as a member.
//  4. Add a policy that lets ada read grace: 201. As ada, remove grace: 204, and maintainers has no members.
func TestGroups_RemoveHiddenMember(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace := createIdentity(api, "grace")
	maintainers, maintainersPath := createGroup(api, "maintainers")
	addMembers(api, maintainersPath, grace)

	// 2. Add the policy.
	addPolicy(t, api, `permit (
		principal == Identity::"`+ada.ID.String()+`",
		action in [Action::"group:read", Action::"group:removeMember"],
		resource == Group::"`+maintainers.ID.String()+`"
	);`)

	// 3. Remove grace.
	assertLikeMissing(t, asAda.Delete,
		maintainersPath+"/members/"+grace.String(), maintainersPath+"/members/"+uuid.NewV4().String(),
		memberNotFound)
	assert.Equal(t, []uuid.UUID{grace}, memberIDs(api, maintainersPath))

	// 4. Remove grace, readable.
	addPolicy(t, api, `permit (
		principal == Identity::"`+ada.ID.String()+`",
		action == Action::"identity:read",
		resource == Identity::"`+grace.String()+`"
	);`)
	asAda.Delete(maintainersPath + "/members/" + grace.String()).AssertNoContent()
	assert.Empty(t, memberIDs(api, maintainersPath))
}

// TestGroups_RemoveLastAdministrator checks that the last member of the administrators group can be
// removed, which locks the admin out, as accepted for now:
//
//  1. List the members of administrators: the admin alone.
//  2. Remove the admin: 204.
//  3. List the groups: an empty 200. Get administrators and add the admin back: 404 group-not-found for
//     both, so nothing restores the access through the API.
func TestGroups_RemoveLastAdministrator(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	admin := adminID(api)
	administratorsPath := groupsPath + "/" + apptest.AdministratorsGroup(t, api).String()

	// 1. List the members.
	assert.Equal(t, []uuid.UUID{admin}, memberIDs(api, administratorsPath))

	// 2. Remove the admin.
	api.Delete(administratorsPath + "/members/" + admin.String()).AssertNoContent()

	// 3. Locked out.
	assert.Empty(t, listGroups(api))
	api.Get(administratorsPath).AssertProblemType(groupNotFound)
	api.Put(administratorsPath+"/members/"+admin.String(), "").AssertProblemType(groupNotFound)
}

// TestGroups_Patch changes a group with JSON Merge Patches, as admin:
//
//  1. Create maintainers and reviewers: 201 for each.
//  2. Patch maintainers' name: 200 with the new name and the update time advanced.
//  3. Patch it with an empty object: 200 with maintainers as step 2 left it, the update time included.
//  4. Patch its name to null: 422 invalid-body pointing to name. To reviewers: 409 group-name-taken, the
//     same as replacing it. Get it: as step 2 left it.
//  5. Patch a group that doesn't exist: 404 group-not-found.
func TestGroups_Patch(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	maintainers, maintainersPath := createGroup(api, "Maintainers")
	createGroup(api, "Reviewers")

	// 2. Patch the name.
	patched := api.Patch(maintainersPath, `{"name":"Core maintainers"}`).DecodeResource[group](http.StatusOK)
	assert.Equal(t, envelope.Visible("Core maintainers"), patched.Name)
	assert.True(t, patched.UpdatedAt.Value.After(maintainers.UpdatedAt.Value), "the update time must advance")

	// 3. Patch nothing.
	assert.Equal(t, patched, api.Patch(maintainersPath, `{}`).DecodeResource[group](http.StatusOK))

	// 4. Patch with invalid names.
	resp := api.Patch(maintainersPath, `{"name":null}`)
	resp.AssertProblemType(invalidBody)
	assert.Contains(t, resp.Body.String(), `"pointer":"#/name"`)
	api.Patch(maintainersPath, `{"name":"Reviewers"}`).AssertProblemType(groupNameTaken)
	assert.Equal(t, patched, api.Get(maintainersPath).DecodeResource[group](http.StatusOK))

	// 5. Patch a missing group.
	api.Patch(groupsPath+"/"+uuid.NewV4().String(), `{}`).AssertProblemType(groupNotFound)
}

// TestGroups_PatchByPolicy checks that patching a group needs only the actions of the fields that the patch
// sets, after reading the group:
//
//  1. Create ada with an API key, and the groups maintainers and hidden: 201 for each.
//  2. Add a policy that lets ada read maintainers: 201.
//  3. As ada, patch maintainers with an empty object: 200 with maintainers, since the patch sets no field.
//     Patch its name: 403 fields-denied pointing to name, and, as admin, maintainers is unchanged.
//  4. As ada, patch hidden, with a name and with an empty object: the same 404 group-not-found, to the byte,
//     as for a group that doesn't exist.
func TestGroups_PatchByPolicy(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	maintainers, maintainersPath := createGroup(api, "Maintainers")
	_, hiddenPath := createGroup(api, "Hidden")
	missingPath := groupsPath + "/" + uuid.NewV4().String()

	// 2. Add the policy.
	addPolicy(t, api, `permit (
		principal == Identity::"`+ada.ID.String()+`",
		action in [Action::"group:read", Action::"group:readFields"],
		resource == Group::"`+maintainers.ID.String()+`"
	);`)

	// 3. Patch maintainers.
	assert.Equal(t, maintainers, asAda.Patch(maintainersPath, `{}`).DecodeResource[group](http.StatusOK))
	assertFieldsDenied(t, asAda.Patch(maintainersPath, `{"name":"Mine"}`), "#/name")
	assert.Equal(t, maintainers, api.Get(maintainersPath).DecodeResource[group](http.StatusOK))

	// 4. Patch hidden.
	for _, body := range []string{`{"name":"Mine"}`, `{}`} {
		assertLikeMissing(t, func(target string) *apitest.Response { return asAda.Patch(target, body) },
			hiddenPath, missingPath, groupNotFound)
	}
}
