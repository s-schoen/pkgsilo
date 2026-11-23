// End-to-end tests of the permissions endpoint and tool: each test drives the whole app through /api/v1 and
// /mcp, as cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps in
// order, each with the result it expects; the numbered comments in the test mark where each step starts.
package permission_test

import (
	"net/http"
	"strconv"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/envelope"
	"pkgsilo/core/mcpserver"
	identityapi "pkgsilo/feature/identity/api"
	permissionapi "pkgsilo/feature/permission/api"
	permissiontools "pkgsilo/feature/permission/tools"
	"pkgsilo/test/apitest"
	"pkgsilo/test/apptest"
)

// The problem types of the permissions endpoint, written out independently of the code: clients tell problems
// apart by them, so a changed slug, title or status must fail these tests.
var (
	identityNotFound = envelope.DefineProblemType("identity-not-found", "Identity not found", http.StatusNotFound)
	invalidQuery     = envelope.DefineProblemType("invalid-query", "Invalid query parameters", http.StatusBadRequest)
	forbidden        = envelope.ProblemType{Type: "about:blank", Title: "Forbidden", Status: http.StatusForbidden}
)

// The tool errors of the permission tool, written out independently of the code: agents tell errors apart by
// their codes and act on their messages, so a changed code or message must fail these tests.
var errIdentityNotFound = mcpserver.DefineError("identity_not_found",
	"The identity does not exist, or the policies do not allow you to read it.")

// The actions that the permissions list, written out independently of the code: clients rely on their names.
var (
	// allActions are every declared action, all of which the administrators group allows, in the order of the
	// types and of their actions.
	allActions = []string{
		"identity:read", "identity:delete",
		"identity:read:username", "identity:read:displayName", "identity:read:createdBy", "identity:read:updatedBy",
		"identity:read:createdAt", "identity:read:updatedAt",
		"identity:create:username", "identity:create:displayName",
		"identity:update:username", "identity:update:displayName",
		"group:read", "group:delete", "group:readMembers", "group:addMember", "group:removeMember",
		"group:read:name", "group:read:createdBy", "group:read:updatedBy", "group:read:createdAt",
		"group:read:updatedAt",
		"group:create:name", "group:update:name",
		"apikey:read", "apikey:delete",
		"apikey:read:identityId", "apikey:read:name", "apikey:read:keySuffix", "apikey:read:expiresAt",
		"apikey:read:lastUsedAt", "apikey:read:createdBy", "apikey:read:createdAt",
		"apikey:create:name", "apikey:create:expiresAt",
		"password:read", "password:delete", "password:change", "password:reset", "password:read:setAt",
		"policy:read", "policy:delete", "policy:readSchema",
		"policy:read:name", "policy:read:source", "policy:read:createdBy", "policy:read:updatedBy",
		"policy:read:createdAt", "policy:read:updatedAt",
		"policy:create:name", "policy:create:source", "policy:update:name", "policy:update:source",
	}
	// ownedActions are the actions for an owner that the administrators group allows: creating its API keys
	// and every action on its password.
	ownedActions = []string{
		"apikey:create:name", "apikey:create:expiresAt",
		"password:read", "password:delete", "password:change", "password:reset", "password:read:setAt",
	}
	// selfServiceActions are the actions that the seeded self-service policies allow an identity outside the
	// administrators group on some resource: reading its own identity, managing its own API keys and reading
	// and changing its own password.
	selfServiceActions = []string{
		"identity:read",
		"identity:read:username", "identity:read:displayName", "identity:read:createdBy", "identity:read:updatedBy",
		"identity:read:createdAt", "identity:read:updatedAt",
		"apikey:read", "apikey:delete",
		"apikey:read:identityId", "apikey:read:name", "apikey:read:keySuffix", "apikey:read:expiresAt",
		"apikey:read:lastUsedAt", "apikey:read:createdBy", "apikey:read:createdAt",
		"apikey:create:name", "apikey:create:expiresAt",
		"password:read", "password:change", "password:read:setAt",
	}
	// ownSelfServiceActions are the actions that the seeded self-service policies allow an identity outside
	// the administrators group for itself as the owner.
	ownSelfServiceActions = []string{
		"apikey:create:name", "apikey:create:expiresAt", "password:read", "password:change", "password:read:setAt",
	}
)

type permissions = permissionapi.PermissionsResponse

// permissionsPathOf returns the path of the permissions of the identity with the given ID or "me".
func permissionsPathOf(id string) string {
	return "/api/v1/identities/" + id + "/permissions"
}

// assertPermissions asserts that the client gets the permissions of the identity with the given ID, which
// allow the actions, at the path.
func assertPermissions(t *testing.T, client *apitest.Client, path string, id uuid.UUID, actions []string) {
	t.Helper()

	assert.Equal(t, permissions{ID: id, AllowedActions: envelope.Visible(actions)},
		client.Get(path).DecodeResource[permissions](http.StatusOK), path)
}

// adminID returns the ID of the identity that the client admin authenticates as.
func adminID(admin *apitest.Client) uuid.UUID {
	return admin.Get("/api/v1/identities/me").DecodeResource[identityapi.IdentityResponse](http.StatusOK).ID
}

// TestPermissions_Admin checks the permissions of admin, whom the administrators group allows everything:
//
//  1. Get admin's permissions by its ID and by "me": every declared action.
//  2. Get them for ada, and for admin itself, as the owner: creating API keys and every action on a password.
//  3. Get ada's permissions, whom admin may read: 403 Forbidden.
//  4. Get them for an owner that doesn't exist: 404 identity-not-found. For a malformed owner: 400
//     invalid-query.
func TestPermissions_Admin(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	admin := adminID(api)
	ada, _ := apptest.NewIdentity(api, "ada")

	// 1. Without owner.
	assertPermissions(t, api, permissionsPathOf(admin.String()), admin, allActions)
	assertPermissions(t, api, permissionsPathOf("me"), admin, allActions)

	// 2. With owner.
	assertPermissions(t, api, permissionsPathOf("me")+"?owner="+ada.ID.String(), admin, ownedActions)
	assertPermissions(t, api, permissionsPathOf("me")+"?owner=me", admin, ownedActions)

	// 3. Another identity.
	api.Get(permissionsPathOf(ada.ID.String())).AssertProblemType(forbidden)

	// 4. A missing and a malformed owner.
	api.Get(permissionsPathOf("me") + "?owner=" + uuid.NewV4().String()).AssertProblemType(identityNotFound)
	api.Get(permissionsPathOf("me") + "?owner=ada").AssertProblemType(invalidQuery)
}

// TestPermissions_Restricted checks the permissions of ada, an identity outside the administrators group,
// whom only the seeded self-service policies allow anything:
//
//  1. Get ada's permissions by her ID and by "me": reading her identity, managing her API keys, reading and
//     changing her password. No create of an identity, group or policy, since those are exact.
//  2. Get them for herself as the owner: creating her API keys, reading and changing her password, but no
//     reset or delete.
//  3. Get them for admin as the owner, whom she may not read: 404 identity-not-found.
//  4. Get admin's permissions, whom she may not read: 404 identity-not-found.
//  5. As admin, add policies that let ada create groups and delete her own identity, then get her
//     permissions: group:create:name and identity:delete join the list.
func TestPermissions_Restricted(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	admin := adminID(api)
	ada, asAda := apptest.NewIdentity(api, "ada")

	// 1. Without owner.
	assertPermissions(t, asAda, permissionsPathOf(ada.ID.String()), ada.ID, selfServiceActions)
	assertPermissions(t, asAda, permissionsPathOf("me"), ada.ID, selfServiceActions)

	// 2. Herself as the owner.
	assertPermissions(t, asAda, permissionsPathOf("me")+"?owner=me", ada.ID, ownSelfServiceActions)
	assertPermissions(t, asAda, permissionsPathOf("me")+"?owner="+ada.ID.String(), ada.ID, ownSelfServiceActions)

	// 3. Admin as the owner.
	asAda.Get(permissionsPathOf("me") + "?owner=" + admin.String()).AssertProblemType(identityNotFound)

	// 4. Admin's permissions.
	asAda.Get(permissionsPathOf(admin.String())).AssertProblemType(identityNotFound)

	// 5. Policies for ada.
	for i, source := range []string{
		`permit (principal == Identity::"` + ada.ID.String() + `", action in Action::"group:create", resource);`,
		`permit (principal == Identity::"` + ada.ID.String() + `", action == Action::"identity:delete", resource) ` +
			`when { resource == principal };`,
	} {
		api.Post("/api/v1/policies", `{"name":"ada-`+strconv.Itoa(i)+`","source":`+strconv.Quote(source)+`}`).
			AssertStatus(http.StatusCreated)
	}
	assertPermissions(t, asAda, permissionsPathOf("me"), ada.ID, []string{
		"identity:read", "identity:delete",
		"identity:read:username", "identity:read:displayName", "identity:read:createdBy", "identity:read:updatedBy",
		"identity:read:createdAt", "identity:read:updatedAt",
		"group:create:name",
		"apikey:read", "apikey:delete",
		"apikey:read:identityId", "apikey:read:name", "apikey:read:keySuffix", "apikey:read:expiresAt",
		"apikey:read:lastUsedAt", "apikey:read:createdBy", "apikey:read:createdAt",
		"apikey:create:name", "apikey:create:expiresAt",
		"password:read", "password:change", "password:read:setAt",
	})
}

// TestPermissions_MCP takes ada through permission_list:
//
//  1. List her permissions: the same as the REST endpoint's.
//  2. List them for herself as the owner: creating her API keys, reading and changing her password.
//  3. List them for admin as the owner, whom she may not read: identity_not_found.
func TestPermissions_MCP(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	admin := adminID(api)
	ada, asAda := apptest.NewIdentity(api, "ada")
	mcp := apptest.MCP(t, asAda)

	// 1. Without owner.
	assert.Equal(t, permissiontools.PermissionsOutput{ID: ada.ID, AllowedActions: envelope.Visible(selfServiceActions)},
		mcp.Call("permission_list", `{}`).Decode[permissiontools.PermissionsOutput]())

	// 2. Herself as the owner.
	assert.Equal(t,
		permissiontools.PermissionsOutput{ID: ada.ID, AllowedActions: envelope.Visible(ownSelfServiceActions)},
		mcp.Call("permission_list", `{"owner":"`+ada.ID.String()+`"}`).Decode[permissiontools.PermissionsOutput]())

	// 3. Admin as the owner.
	mcp.Call("permission_list", `{"owner":"`+admin.String()+`"}`).AssertToolError(errIdentityNotFound)
}
