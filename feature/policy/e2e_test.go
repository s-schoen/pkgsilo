// End-to-end tests of the policy endpoints: each test drives the whole app through /api/v1, as
// cmd/main.go serves it, on a fresh database. The doc comment of each test lists its steps in order,
// each with the result it expects; the numbered comments in the test mark where each step starts.
package policy_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/envelope"
	groupapi "pkgsilo/feature/group/api"
	identityapi "pkgsilo/feature/identity/api"
	policyapi "pkgsilo/feature/policy/api"
	"pkgsilo/test/apitest"
	"pkgsilo/test/apptest"
)

// The problem types of the policy endpoints, written out independently of the code: clients tell
// problems apart by them, so a changed slug, title or status must fail these tests.
var (
	policyNotFound   = envelope.DefineProblemType("policy-not-found", "Policy not found", http.StatusNotFound)
	policyNameTaken  = envelope.DefineProblemType("policy-name-taken", "Policy name taken", http.StatusConflict)
	invalidPolicy    = envelope.DefineProblemType("invalid-policy", "Invalid policy", http.StatusUnprocessableEntity)
	identityNotFound = envelope.DefineProblemType("identity-not-found", "Identity not found", http.StatusNotFound)
	fieldsDenied     = envelope.DefineProblemType("fields-denied", "Fields denied", http.StatusForbidden)
	invalidBody      = envelope.DefineProblemType("invalid-body", "Invalid request body", http.StatusUnprocessableEntity)
	forbidden        = envelope.ProblemType{Type: "about:blank", Title: "Forbidden", Status: http.StatusForbidden}
)

const (
	policiesPath   = "/api/v1/policies"
	identitiesPath = "/api/v1/identities"
	groupsPath     = "/api/v1/groups"
)

// administratorsGroup is the ID of the group that the seeded administrators policy grants everything.
const administratorsGroup = "7c1d3e52-8a4f-4b6e-9f21-5d0c8e7a3b14"

type (
	policy   = policyapi.PolicyResponse
	identity = identityapi.IdentityResponse
)

// policyBody returns the JSON body of a policy with the given name and source.
func policyBody(t *testing.T, name, source string) string {
	t.Helper()

	body, err := json.Marshal(map[string]string{"name": name, "source": source})
	require.NoError(t, err)

	return string(body)
}

// addPolicy creates a policy through the client and returns it.
func addPolicy(t *testing.T, api *apitest.Client, name, source string) policy {
	t.Helper()

	return apitest.DecodeResource[policy](api.Post(policiesPath, policyBody(t, name, source)), http.StatusCreated)
}

// addGroup creates a group with the identities as members through the client and returns its ID.
func addGroup(api *apitest.Client, name string, members ...uuid.UUID) string {
	group := apitest.DecodeResource[groupapi.GroupResponse](
		api.Post(groupsPath, `{"name":"`+name+`"}`), http.StatusCreated)
	for _, member := range members {
		api.Put(groupsPath+"/"+group.ID.String()+"/members/"+member.String(), "").AssertNoContent()
	}

	return group.ID.String()
}

// listPolicies returns all policies that the client may read.
func listPolicies(api *apitest.Client) []policy {
	return apitest.DecodeCollection[policy](api.Get(policiesPath), http.StatusOK).Data
}

// assertInvalidPolicy asserts an invalid-policy problem whose detail contains reason.
func assertInvalidPolicy(t *testing.T, resp *apitest.Response, reason string) {
	t.Helper()
	resp.AssertProblemType(invalidPolicy)

	var problem struct {
		Detail string `json:"detail"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &problem))
	assert.Contains(t, problem.Detail, reason)
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

// TestPolicies_Lifecycle takes one policy through every endpoint:
//
//  1. Create a policy: 201 with the new policy, its Location under /api/v1/policies, and its audit
//     fields set by the server.
//  2. Get it through the Location: the created policy.
//  3. List the policies: the seeded administrators, read-own-identity and manage-own-api-keys policies, then
//     the new one.
//  4. Replace its name and source: 200 with the new values, the ID and creation time kept and the
//     update time advanced.
//  5. Get it again: the replaced policy, so the change was stored.
//  6. Delete it: 204.
//  7. Get, replace and delete it again: 404 policy-not-found for each.
func TestPolicies_Lifecycle(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	const source = `permit (principal, action == Action::"identity:read", resource);`

	// 1. Create.
	created := api.Post(policiesPath, policyBody(t, "read-identities", source))
	readIdentities := apitest.DecodeResource[policy](created, http.StatusCreated)
	location := policiesPath + "/" + readIdentities.ID.String()
	assert.Equal(t, location, created.Header().Get("Location"))
	assert.Equal(t, envelope.Visible("read-identities"), readIdentities.Name)
	assert.Equal(t, envelope.Visible(source), readIdentities.Source)
	assert.Equal(t, readIdentities.CreatedBy, readIdentities.UpdatedBy)
	assert.WithinDuration(t, time.Now(), readIdentities.CreatedAt.Value, time.Minute)
	assert.Equal(t, readIdentities.CreatedAt, readIdentities.UpdatedAt)

	// 2. Get.
	assert.Equal(t, readIdentities, apitest.DecodeResource[policy](api.Get(location), http.StatusOK))

	// 3. List.
	listed := listPolicies(api)
	require.Len(t, listed, 4)
	assert.Equal(t, envelope.Visible("administrators"), listed[0].Name)
	assert.Equal(t, envelope.Visible("read-own-identity"), listed[1].Name)
	assert.Equal(t, envelope.Visible("manage-own-api-keys"), listed[2].Name)
	assert.Equal(t, readIdentities, listed[3])

	// 4. Replace.
	const replacedSource = `permit (principal, action in Action::"read", resource);`
	replaced := apitest.DecodeResource[policy](
		api.Put(location, policyBody(t, "read-everything", replacedSource)), http.StatusOK)
	assert.Equal(t, readIdentities.ID, replaced.ID)
	assert.Equal(t, envelope.Visible("read-everything"), replaced.Name)
	assert.Equal(t, envelope.Visible(replacedSource), replaced.Source)
	assert.Equal(t, readIdentities.CreatedAt, replaced.CreatedAt, "the creation time is immutable")
	assert.True(t, replaced.UpdatedAt.Value.After(readIdentities.UpdatedAt.Value), "the update time must advance")

	// 5. Get again.
	assert.Equal(t, replaced, apitest.DecodeResource[policy](api.Get(location), http.StatusOK))

	// 6. Delete.
	api.Delete(location).AssertNoContent()

	// 7. Get, replace and delete again.
	api.Get(location).AssertProblemType(policyNotFound)
	api.Put(location, policyBody(t, "read-everything", replacedSource)).AssertProblemType(policyNotFound)
	api.Delete(location).AssertProblemType(policyNotFound)
}

// TestPolicies_NameTaken checks that the database's unique name constraint reaches clients as a conflict:
//
//  1. Create the policy one: 201.
//  2. Create another policy named one, and rename the seeded read-own-identity to one: 409
//     policy-name-taken for both.
func TestPolicies_NameTaken(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	const source = `permit (principal, action == Action::"identity:read", resource);`

	// 1. Create.
	addPolicy(t, api, "one", source)

	// 2. Create and rename.
	api.Post(policiesPath, policyBody(t, "one", source)).AssertProblemType(policyNameTaken)
	readOwn := listPolicies(api)[1]
	api.Put(policiesPath+"/"+readOwn.ID.String(), policyBody(t, "one", readOwn.Source.Value)).
		AssertProblemType(policyNameTaken)
}

// TestPolicies_Invalid checks that a policy with a mistake is rejected, with a reason, both when it's
// created and when it replaces a valid one:
//
//  1. Create the group maintainers, and a valid policy to replace: 201 for both.
//  2. For each mistake, create a policy with it: 422 invalid-policy with the reason in the detail.
//  3. Replace the valid policy with it: the same 422.
//  4. Get the valid policy, and list the policies: the valid policy unchanged, and no policy named after the
//     mistake, so nothing was written.
//
// Steps 2 to 4 are parallel subtests on the same app, one for each mistake.
func TestPolicies_Invalid(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	maintainers := addGroup(api, "maintainers")
	valid := addPolicy(t, api, "valid",
		`permit (principal in Group::"`+maintainers+`", action in Action::"read", resource);`)
	validPath := policiesPath + "/" + valid.ID.String()

	tests := []struct {
		name, source, reason string
	}{
		{name: "parse error", source: `permit (principal, action, resource`, reason: "can't be parsed"},
		{
			name:   "oversized",
			source: `permit (principal, action, resource);` + strings.Repeat(" ", 16<<10),
			reason: "larger than 16384 bytes",
		},
		{name: "no statements", source: `// Nothing yet.`, reason: "holds no statements"},
		{
			name:   "typo in an action",
			source: `permit (principal, action == Action::"identity:raed", resource);`,
			reason: "unrecognized action `Action::\"identity:raed\"`",
		},
		{
			name:   "typo in an entity type",
			source: `permit (principal, action == Action::"identity:read", resource is Identiy);`,
			reason: "unrecognized entity type `Identiy`",
		},
		{
			name:   "typo in an attribute",
			source: `permit (principal, action == Action::"identity:read", resource) when { resource.usernmae == "ada" };`,
			reason: "attribute `usernmae` on entity type `Identity` not found",
		},
		{
			name:   "action that doesn't apply to the resource",
			source: `permit (principal, action == Action::"policy:read", resource is Identity);`,
			reason: "unable to find an applicable action",
		},
		{
			name:   "action group compared with ==",
			source: `permit (principal, action == Action::"read", resource);`,
			reason: "unable to find an applicable action",
		},
		{
			name:   "guarded typo in a forbid",
			source: `forbid (principal, action, resource) when { resource has secrte && resource.secrte };`,
			reason: `statement 1 tests for the attribute "secrte" with has`,
		},
		{
			name: "guarded typo in a later statement",
			source: `permit (principal, action == Action::"identity:read", resource);
				forbid (principal, action, resource) when { context has secrte };`,
			reason: `statement 2 tests for the attribute "secrte" with has`,
		},
		{
			name:   "group that doesn't exist",
			source: `forbid (principal in Group::"` + uuid.NewV4().String() + `", action, resource);`,
			reason: "does not exist",
		},
		{
			name:   "group ID that isn't a UUID",
			source: `forbid (principal in Group::"maintainers", action, resource);`,
			reason: `the group "maintainers" does not exist`,
		},
		{
			name: "group that doesn't exist, next to one that does",
			source: `permit (principal in Group::"` + maintainers + `", action in Action::"read", resource)
				when { principal in [Group::"` + maintainers + `", Group::"0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4"] };`,
			reason: `the group "0b6d7c1e-2f3a-4b5c-9d6e-7f8091a2b3c4" does not exist`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api := api.For(t)

			// 2. Create.
			assertInvalidPolicy(t, api.Post(policiesPath, policyBody(t, tt.name, tt.source)), tt.reason)

			// 3. Replace.
			assertInvalidPolicy(t, api.Put(validPath, policyBody(t, tt.name, tt.source)), tt.reason)

			// 4. Nothing written.
			assert.Equal(t, valid, apitest.DecodeResource[policy](api.Get(validPath), http.StatusOK))
			for _, p := range listPolicies(api) {
				assert.NotEqual(t, tt.name, p.Name.Value)
			}
		})
	}
}

// statements returns a source of n statements that never match.
func statements(n int) string {
	return strings.Repeat(`permit(principal,action,resource)when{false};`, n)
}

// TestPolicies_StatementLimit checks that all policies together hold no more than 1,000 statements:
//
//  1. Create two policies of 360 statements each, and one of 277: 201 for each, since the seeded policies
//     hold three, which makes 1,000.
//  2. Create a policy of one statement: 422 invalid-policy.
//  3. Replace the policy of 277 statements with 278: 422 invalid-policy. Replace it with itself: 200, since
//     its own statements are counted once.
//  4. Replace the policy of 277 statements with 276, and create the policy of one statement again: 200 and
//     201.
func TestPolicies_StatementLimit(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	const reason = "the policies would hold 1001 statements, more than the 1000"

	// 1. Create.
	addPolicy(t, api, "first", statements(360))
	addPolicy(t, api, "second", statements(360))
	third := addPolicy(t, api, "third", statements(277))
	thirdPath := policiesPath + "/" + third.ID.String()

	// 2. Create one more statement.
	assertInvalidPolicy(t, api.Post(policiesPath, policyBody(t, "one", statements(1))), reason)

	// 3. Replace.
	assertInvalidPolicy(t, api.Put(thirdPath, policyBody(t, "third", statements(278))), reason)
	api.Put(thirdPath, policyBody(t, "third", statements(277))).AssertStatus(http.StatusOK)

	// 4. Make room.
	api.Put(thirdPath, policyBody(t, "third", statements(276))).AssertStatus(http.StatusOK)
	addPolicy(t, api, "one", statements(1))
}

// TestPolicies_SeededPassChecks checks that the seeded policies pass the checks of a saved policy:
//
//  1. List the policies: the seeded administrators, read-own-identity and manage-own-api-keys policies.
//  2. Replace each with itself: 200.
func TestPolicies_SeededPassChecks(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. List.
	seeded := listPolicies(api)
	require.Len(t, seeded, 3)

	// 2. Replace.
	for _, p := range seeded {
		api.Put(policiesPath+"/"+p.ID.String(), policyBody(t, p.Name.Value, p.Source.Value)).AssertStatus(http.StatusOK)
	}
}

// TestPolicies_ProtectedByPolicies checks that the policy endpoints follow the policies, which give an
// identity outside the administrators group no access to them:
//
//  1. Create ada with an API key: 201.
//  2. As ada, list the policies: an empty 200.
//  3. As ada, get, replace and delete the administrators policy: the same 404 policy-not-found, to the byte,
//     as for a policy that doesn't exist.
//  4. As ada, create a policy that would give her everything: 403 fields-denied pointing to name and
//     source, and, as admin, no new policy in the list.
func TestPolicies_ProtectedByPolicies(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)
	administrators := listPolicies(api)[0]
	administratorsPath := policiesPath + "/" + administrators.ID.String()
	missingPath := policiesPath + "/" + uuid.NewV4().String()

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")

	// 2. List.
	assert.Empty(t, listPolicies(asAda))

	// 3. Get, replace and delete.
	body := policyBody(t, "administrators", administrators.Source.Value)
	for _, send := range []func(target string) *apitest.Response{
		asAda.Get,
		func(target string) *apitest.Response { return asAda.Put(target, body) },
		asAda.Delete,
	} {
		hidden := send(administratorsPath)
		hidden.AssertProblemType(policyNotFound)
		assert.Equal(t, send(missingPath).Body.String(), hidden.Body.String())
	}

	// 4. Create.
	assertFieldsDenied(t,
		asAda.Post(policiesPath,
			policyBody(t, "ada", `permit (principal == Identity::"`+ada.ID.String()+`", action, resource);`)),
		"#/name", "#/source")
	assert.Len(t, listPolicies(api), 3)
}

// TestPolicies_SecondInstance checks that a policy change applies on the next request of every instance on
// the database, not only of the one that made it:
//
//  1. Create ada with an API key, and grace: 201 for each.
//  2. On the second instance, as ada, get grace: 404 identity-not-found, so the second instance has loaded
//     the policies.
//  3. On the first instance, create a policy that lets ada read grace: 201.
//  4. On the second instance, as ada, get grace: 200.
//  5. On the first instance, delete the policy: 204.
//  6. On the second instance, as ada, get grace: 404 identity-not-found again.
func TestPolicies_SecondInstance(t *testing.T) {
	t.Parallel()
	api, db := apptest.NewWithDB(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace, _ := apptest.NewIdentity(api, "grace")
	gracePath := identitiesPath + "/" + grace.ID.String()
	asAdaElsewhere := apptest.NewInstance(t, asAda, db)

	// 2. Get grace elsewhere.
	asAdaElsewhere.Get(gracePath).AssertProblemType(identityNotFound)

	// 3. Create the policy.
	readGrace := addPolicy(t, api, "ada-reads-grace", `permit (
		principal == Identity::"`+ada.ID.String()+`",
		action in [Action::"identity:read", Action::"identity:readFields"],
		resource == Identity::"`+grace.ID.String()+`"
	);`)

	// 4. Get grace elsewhere.
	assert.Equal(t, grace, apitest.DecodeResource[identity](asAdaElsewhere.Get(gracePath), http.StatusOK))

	// 5. Delete the policy.
	api.Delete(policiesPath + "/" + readGrace.ID.String()).AssertNoContent()

	// 6. Get grace elsewhere.
	asAdaElsewhere.Get(gracePath).AssertProblemType(identityNotFound)
}

// TestPolicies_IdentitiesButNotOtherTypes checks a policy that limits access by object type: the
// members of a group manage identities, but nothing else, here policies. The group tests cover groups:
//
//  1. Create ada with an API key, and the group identity-managers with her as a member: 201 and 204.
//  2. Add a policy that lets identity-managers read, create, replace and delete identities: 201.
//  3. As ada, list the identities: all three. Create carol, replace her and delete her: 201, 200 and 204.
//  4. As ada, list the policies: an empty 200. Get the administrators policy: 404 policy-not-found. Create
//     a policy: 403 fields-denied pointing to name and source.
func TestPolicies_IdentitiesButNotOtherTypes(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	managers := addGroup(api, "identity-managers", ada.ID)

	// 2. Add the policy.
	addPolicy(t, api, "identity-managers", `permit (
		principal in Group::"`+managers+`",
		action in [Action::"read", Action::"identity:create", Action::"identity:update", Action::"identity:delete"],
		resource is Identity
	);`)

	// 3. Manage identities.
	assert.Len(t, apitest.DecodeCollection[identity](asAda.Get(identitiesPath), http.StatusOK).Data, 2)
	carol := apitest.DecodeResource[identity](
		asAda.Post(identitiesPath, `{"username":"carol","displayName":"Carol"}`), http.StatusCreated)
	carolPath := identitiesPath + "/" + carol.ID.String()
	asAda.Put(carolPath, `{"username":"carol","displayName":"Carol Shaw"}`).AssertStatus(http.StatusOK)
	asAda.Delete(carolPath).AssertNoContent()

	// 4. Policies.
	assert.Empty(t, listPolicies(asAda))
	asAda.Get(policiesPath + "/" + listPolicies(api)[0].ID.String()).AssertProblemType(policyNotFound)
	assertFieldsDenied(t, asAda.Post(policiesPath, policyBody(t, "ada", `permit (principal, action, resource);`)),
		"#/name", "#/source")
}

// TestPolicies_ReadOnlyAudit checks a policy that limits access by method: the members of an audit group
// read everything and write nothing.
//
//  1. Create ada with an API key, grace, and the group auditors with ada as a member: 201 and 204.
//  2. Add a policy that lets auditors read everything: 201.
//  3. As ada, list the identities and the policies: all of them, the same as admin, with every field. Get
//     grace and the new policy: 200 with every field.
//  4. As ada, create an identity and a policy: 403 fields-denied for both. Replace grace and the new
//     policy: 403 fields-denied for both. Delete grace and the new policy: 403 Forbidden for both.
//  5. As admin, list the identities and the policies: nothing was written.
func TestPolicies_ReadOnlyAudit(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace, _ := apptest.NewIdentity(api, "grace")
	gracePath := identitiesPath + "/" + grace.ID.String()
	auditors := addGroup(api, "auditors", ada.ID)

	// 2. Add the policy.
	audit := addPolicy(t, api, "auditors",
		`permit (principal in Group::"`+auditors+`", action in Action::"read", resource);`)
	auditPath := policiesPath + "/" + audit.ID.String()

	// 3. Read.
	identities := apitest.DecodeCollection[identity](api.Get(identitiesPath), http.StatusOK).Data
	policies := listPolicies(api)
	assert.Equal(t, identities, apitest.DecodeCollection[identity](asAda.Get(identitiesPath), http.StatusOK).Data)
	assert.Equal(t, policies, listPolicies(asAda))
	assert.Equal(t, grace, apitest.DecodeResource[identity](asAda.Get(gracePath), http.StatusOK))
	assert.Equal(t, audit, apitest.DecodeResource[policy](asAda.Get(auditPath), http.StatusOK))

	// 4. Write.
	assertFieldsDenied(t, asAda.Post(identitiesPath, `{"username":"carol","displayName":"Carol"}`),
		"#/username", "#/displayName")
	assertFieldsDenied(t, asAda.Post(policiesPath, policyBody(t, "more", audit.Source.Value)), "#/name", "#/source")
	assertFieldsDenied(t, asAda.Put(gracePath, `{"username":"grace","displayName":"Grace"}`),
		"#/username", "#/displayName")
	assertFieldsDenied(t, asAda.Put(auditPath, policyBody(t, "auditors", audit.Source.Value)), "#/name", "#/source")
	asAda.Delete(gracePath).AssertProblemType(forbidden)
	asAda.Delete(auditPath).AssertProblemType(forbidden)

	// 5. Nothing written.
	assert.Equal(t, identities, apitest.DecodeCollection[identity](api.Get(identitiesPath), http.StatusOK).Data)
	assert.Equal(t, policies, listPolicies(api))
}

// TestPolicies_FieldDenials checks that field rules apply to replacing an identity, whether a grant leaves
// out a field or a forbid takes it away from a grant of all fields:
//
//  1. Create ada with an API key, grace, and the groups display-name-editors and editors: 201 and 204.
//  2. Add a policy that lets display-name-editors read identities and update their displayName alone,
//     and one of two statements that lets editors read and update identities but forbids them to update
//     usernames: 201 for both.
//  3. With ada in display-name-editors, replace grace: 403 fields-denied pointing to username alone, and
//     grace is unchanged.
//  4. With ada in editors instead, replace grace: the same 403, since the forbid overrides the grant.
//  5. Delete the second policy's forbid: 200. Replace grace: 200 without her username, since the policy
//     doesn't let ada read it. As admin, get grace: her new username.
func TestPolicies_FieldDenials(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	grace, _ := apptest.NewIdentity(api, "grace")
	gracePath := identitiesPath + "/" + grace.ID.String()
	displayNameEditors := addGroup(api, "display-name-editors", ada.ID)
	editors := addGroup(api, "editors")

	// 2. Add the policies.
	addPolicy(t, api, "display-name-editors", `permit (
		principal in Group::"`+displayNameEditors+`",
		action in [Action::"identity:read", Action::"identity:update:displayName"],
		resource is Identity
	);`)
	grant := `permit (
		principal in Group::"` + editors + `",
		action in [Action::"identity:read", Action::"identity:update"],
		resource
	);`
	editorsPolicy := addPolicy(t, api, "editors", grant+`
		forbid (principal in Group::"`+editors+`", action == Action::"identity:update:username", resource);`)

	// 3. Replace as a display name editor.
	const body = `{"username":"grace-hopper","displayName":"Grace Hopper"}`
	assertFieldsDenied(t, asAda.Put(gracePath, body), "#/username")
	assert.Equal(t, grace, apitest.DecodeResource[identity](api.Get(gracePath), http.StatusOK))

	// 4. Replace as an editor.
	api.Delete(groupsPath + "/" + displayNameEditors + "/members/" + ada.ID.String()).AssertNoContent()
	api.Put(groupsPath+"/"+editors+"/members/"+ada.ID.String(), "").AssertNoContent()
	assertFieldsDenied(t, asAda.Put(gracePath, body), "#/username")
	assert.Equal(t, grace, apitest.DecodeResource[identity](api.Get(gracePath), http.StatusOK))

	// 5. Without the forbid.
	api.Put(policiesPath+"/"+editorsPolicy.ID.String(), policyBody(t, "editors", grant)).AssertStatus(http.StatusOK)
	replaced := apitest.DecodeResource[identity](asAda.Put(gracePath, body), http.StatusOK)
	assert.Equal(t, identity{ID: grace.ID}, replaced, "the response to a write is masked like a read")
	assert.Equal(t, envelope.Visible("grace-hopper"),
		apitest.DecodeResource[identity](api.Get(gracePath), http.StatusOK).Username)
}

// TestPolicies_DeleteReferencedGroup checks that a group stays deletable while a policy references it:
//
//  1. Create the group maintainers, and a policy that references it: 201 for both.
//  2. Delete maintainers: 204.
//  3. Get the policy: 200, unchanged. Replace it with itself: 422 invalid-policy, since the group no longer
//     exists.
func TestPolicies_DeleteReferencedGroup(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	maintainers := addGroup(api, "maintainers")
	referencing := addPolicy(t, api, "maintainers",
		`permit (principal in Group::"`+maintainers+`", action in Action::"read", resource);`)
	referencingPath := policiesPath + "/" + referencing.ID.String()

	// 2. Delete the group.
	api.Delete(groupsPath + "/" + maintainers).AssertNoContent()

	// 3. The policy.
	assert.Equal(t, referencing, apitest.DecodeResource[policy](api.Get(referencingPath), http.StatusOK))
	assertInvalidPolicy(t, api.Put(referencingPath, policyBody(t, referencing.Name.Value, referencing.Source.Value)),
		`the group "`+maintainers+`" does not exist`)
}

// TestPolicies_DeleteAdministrators checks that the administrators policy can be deleted, which locks the
// admin out, as accepted for now:
//
//  1. Get the administrators policy: 200, granting everything to the administrators group.
//  2. Delete it: 204.
//  3. List the policies: an empty 200. List the identities: the admin alone, through the read-own-identity
//     policy. Create a policy: 403 fields-denied, so nothing restores the access through the API.
func TestPolicies_DeleteAdministrators(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Get.
	administrators := listPolicies(api)[0]
	assert.Equal(t, envelope.Visible("administrators"), administrators.Name)
	assert.Contains(t, administrators.Source.Value, `Group::"`+administratorsGroup+`"`)

	// 2. Delete.
	api.Delete(policiesPath + "/" + administrators.ID.String()).AssertNoContent()

	// 3. Locked out.
	assert.Empty(t, listPolicies(api))
	identities := apitest.DecodeCollection[identity](api.Get(identitiesPath), http.StatusOK).Data
	require.Len(t, identities, 1)
	assert.Equal(t, envelope.Visible("admin"), identities[0].Username)
	assertFieldsDenied(t, api.Post(policiesPath, policyBody(t, "administrators", administrators.Source.Value)),
		"#/name", "#/source")
}

// revision returns the current policy revision, which every policy write advances.
func revision(t *testing.T, db *sqlx.DB) int64 {
	t.Helper()

	var rev int64
	require.NoError(t, db.GetContext(t.Context(), &rev, `SELECT revision FROM policy_revision`))

	return rev
}

// TestPolicies_Patch changes a policy with JSON Merge Patches, as admin, each patch checked like a saved
// policy:
//
//  1. Create the group maintainers, and a policy that references it: 201 for both.
//  2. Patch the policy's name: 200 with the new name, its source kept and the update time advanced. The
//     revision advanced.
//  3. Patch it with an empty object: 200 with the policy as step 2 left it, the update time included. The
//     revision didn't change.
//  4. Patch its source with a typo in an action: 422 invalid-policy naming the action. Patch its name to null:
//     422 invalid-body pointing to name. To administrators: 409 policy-name-taken, the same as replacing it.
//     Get it: as step 2 left it.
//  5. Delete maintainers: 204. Patch the policy's name: 422 invalid-policy, since the source that the patch
//     keeps references a group that no longer exists.
//  6. Patch a policy that doesn't exist: 404 policy-not-found.
func TestPolicies_Patch(t *testing.T) {
	t.Parallel()
	api, db := apptest.NewWithDB(t)

	// 1. Create.
	maintainers := addGroup(api, "maintainers")
	created := addPolicy(t, api, "maintainers",
		`permit (principal in Group::"`+maintainers+`", action in Action::"read", resource);`)
	createdPath := policiesPath + "/" + created.ID.String()

	// 2. Patch the name.
	before := revision(t, db)
	patched := apitest.DecodeResource[policy](api.Patch(createdPath, `{"name":"maintainers-read"}`), http.StatusOK)
	assert.Equal(t, envelope.Visible("maintainers-read"), patched.Name)
	assert.Equal(t, created.Source, patched.Source, "a member that the patch leaves out must not change")
	assert.True(t, patched.UpdatedAt.Value.After(created.UpdatedAt.Value), "the update time must advance")
	assert.Equal(t, before+1, revision(t, db), "a policy write must advance the revision")

	// 3. Patch nothing.
	assert.Equal(t, patched, apitest.DecodeResource[policy](api.Patch(createdPath, `{}`), http.StatusOK))
	assert.Equal(t, before+1, revision(t, db), "a patch that sets nothing writes nothing")

	// 4. Patch with invalid bodies.
	assertInvalidPolicy(t, api.Patch(createdPath, policyPatch(t, "source",
		`permit (principal, action == Action::"policy:raed", resource);`)), "policy:raed")
	resp := api.Patch(createdPath, `{"name":null}`)
	resp.AssertProblemType(invalidBody)
	assert.Contains(t, resp.Body.String(), `"pointer":"#/name"`)
	api.Patch(createdPath, `{"name":"administrators"}`).AssertProblemType(policyNameTaken)
	assert.Equal(t, patched, apitest.DecodeResource[policy](api.Get(createdPath), http.StatusOK))

	// 5. Patch after deleting the group.
	api.Delete(groupsPath + "/" + maintainers).AssertNoContent()
	assertInvalidPolicy(t, api.Patch(createdPath, `{"name":"renamed"}`), `the group "`+maintainers+`" does not exist`)

	// 6. Patch a missing policy.
	api.Patch(policiesPath+"/"+uuid.NewV4().String(), `{}`).AssertProblemType(policyNotFound)
}

// policyPatch returns the JSON Merge Patch that sets the member to value.
func policyPatch(t *testing.T, member, value string) string {
	t.Helper()

	body, err := json.Marshal(map[string]string{member: value})
	require.NoError(t, err)

	return string(body)
}

// TestPolicies_PatchFieldDenials checks that a caller who may set the name of policies but not their source
// can patch the name, but not the source, and still can't replace a policy:
//
//  1. Create ada with an API key, and the policy target: 201 for both.
//  2. Add a policy that lets ada read policies and update their names: 201.
//  3. As ada, patch target's name: 200 with the new name.
//  4. As ada, patch its source, and its name and source: 403 fields-denied pointing to source alone for
//     both. Replace it with its current source: 403 fields-denied pointing to source.
//  5. As admin, get target: as step 3 left it, so nothing else was written.
func TestPolicies_PatchFieldDenials(t *testing.T) {
	t.Parallel()
	api := apptest.New(t)

	// 1. Create.
	ada, asAda := apptest.NewIdentity(api, "ada")
	const source = `permit (principal, action == Action::"identity:read", resource) when { false };`
	target := addPolicy(t, api, "target", source)
	targetPath := policiesPath + "/" + target.ID.String()

	// 2. Add the policy.
	addPolicy(t, api, "rename-policies", `permit (
		principal == Identity::"`+ada.ID.String()+`",
		action in [Action::"policy:read", Action::"policy:readFields", Action::"policy:update:name"],
		resource
	);`)

	// 3. Patch the name.
	patched := apitest.DecodeResource[policy](asAda.Patch(targetPath, `{"name":"renamed"}`), http.StatusOK)
	assert.Equal(t, envelope.Visible("renamed"), patched.Name)
	assert.Equal(t, envelope.Visible(source), patched.Source)

	// 4. Patch and replace the source.
	const other = `permit (principal, action == Action::"identity:read", resource);`
	assertFieldsDenied(t, asAda.Patch(targetPath, policyPatch(t, "source", other)), "#/source")
	assertFieldsDenied(t, asAda.Patch(targetPath, policyBody(t, "again", other)), "#/source")
	assertFieldsDenied(t, asAda.Put(targetPath, policyBody(t, "again", source)), "#/source")

	// 5. Nothing else written.
	assert.Equal(t, patched, apitest.DecodeResource[policy](api.Get(targetPath), http.StatusOK))
}
