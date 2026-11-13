package authz

import (
	"testing"
	"time"
	"uuid"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"pkgsilo/core/database"
	"pkgsilo/test/databasetest"
	"pkgsilo/test/telemetrytest"
)

// insertIdentity stores an identity row for memberships to refer to and returns its ID. The table belongs
// to the identity feature, so the row is written directly.
func insertIdentity(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()

	id := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO identities (id, username, display_name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, 'Test', $1, $1, $3, $3)`,
		id, "identity-"+id.String(), time.Now(),
	)
	require.NoError(t, err)

	return id
}

// insertGroup stores a group row with the identities as members and returns its ID. The tables belong to
// the group feature, so the rows are written directly.
func insertGroup(t *testing.T, db *sqlx.DB, members ...uuid.UUID) uuid.UUID {
	t.Helper()

	id := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO groups (id, name, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $1, $1, $3, $3)`,
		id, "group-"+id.String(), time.Now(),
	)
	require.NoError(t, err)
	addMembers(t, db, id, members...)

	return id
}

// addMembers stores the membership rows of the identities in the group.
func addMembers(t *testing.T, db *sqlx.DB, group uuid.UUID, members ...uuid.UUID) {
	t.Helper()

	for _, member := range members {
		_, err := db.ExecContext(t.Context(), `
			INSERT INTO group_members (group_id, identity_id, created_by, created_at) VALUES ($1, $2, $2, $3)`,
			group, member, time.Now(),
		)
		require.NoError(t, err)
	}
}

// insertPolicyRow stores a policy without announcing it, and returns its ID.
func insertPolicyRow(t *testing.T, db *sqlx.DB, source string) uuid.UUID {
	t.Helper()

	id := uuid.NewV4()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO policies (id, name, source, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $1, $1, $4, $4)`,
		id, "policy-"+id.String(), source, time.Now(),
	)
	require.NoError(t, err)

	return id
}

// bumpRevision announces a policy change, like every policy write does in its transaction.
func bumpRevision(t *testing.T, db *sqlx.DB) {
	t.Helper()

	_, err := db.ExecContext(t.Context(), `UPDATE policy_revision SET revision = revision + 1`)
	require.NoError(t, err)
}

// addPolicy stores a policy and announces it, and returns its ID.
func addPolicy(t *testing.T, db *sqlx.DB, source string) uuid.UUID {
	t.Helper()

	id := insertPolicyRow(t, db, source)
	bumpRevision(t, db)

	return id
}

// removeSeededPolicies removes the seeded policies, so that a test's own policies are the only ones.
func removeSeededPolicies(t *testing.T, db *sqlx.DB) {
	t.Helper()

	_, err := db.ExecContext(t.Context(), `DELETE FROM policies`)
	require.NoError(t, err)
	bumpRevision(t, db)
}

// newEngine returns an Engine on db that records neither metrics nor spans.
func newEngine(db database.Querier) *Engine {
	return New(db, noop.NewMeterProvider(), tracenoop.NewTracerProvider())
}

// assertCan asserts the decision of a new Decider for the actor about the identity resource.
func assertCan(t *testing.T, want bool, engine *Engine, actor uuid.UUID, action Action, resource Resource) {
	t.Helper()

	decider, err := engine.For(t.Context(), actor)
	require.NoError(t, err)

	can, err := decider.Can(action, resource)
	require.NoError(t, err)
	assert.Equal(t, want, can)
}

func identityResource(id uuid.UUID, groups ...uuid.UUID) Resource {
	return Resource{Type: IdentityType, ID: id, Groups: groups}
}

func TestEngine_SeededPolicies(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	engine := newEngine(db)
	ada := insertIdentity(t, db)
	grace := insertIdentity(t, db)
	otherAction := NewAction("identity:delete")
	readKey := NewAction("apikey:read", ActionGroupRead)
	deleteKey := NewAction("apikey:delete")
	createKeyName := NewAction("apikey:create:name", "apikey:create")

	tests := []struct {
		name     string
		actor    uuid.UUID
		action   Action
		resource Resource
		want     bool
	}{
		{"an identity reads itself", ada, testAction, identityResource(ada), true},
		{"an identity doesn't read others", ada, testAction, identityResource(grace), false},
		{"an identity does nothing else to itself", ada, otherAction, identityResource(ada), false},
		{"reading itself covers only identities", ada, testAction, Resource{Type: "Other", ID: ada}, false},
		// Bootstrap gets its access from AllowAll, so that this actor is nobody special at runtime.
		{"the system actor has no access", uuid.Nil(), testAction, identityResource(ada), false},
		{"the system actor reads itself only", uuid.Nil(), testAction, identityResource(uuid.Nil()), true},
		{"the system actor does nothing else", uuid.Nil(), otherAction, identityResource(uuid.Nil()), false},
		{"an identity reads its own api keys", ada, readKey, ownedResource(ada), true},
		{"an identity deletes its own api keys", ada, deleteKey, ownedResource(ada), true},
		{"an identity creates its own api keys", ada, createKeyName, ownedResource(ada), true},
		{"an identity doesn't read others' api keys", ada, readKey, ownedResource(grace), false},
		{"an identity doesn't delete others' api keys", ada, deleteKey, ownedResource(grace), false},
		{"an identity doesn't create others' api keys", ada, createKeyName, ownedResource(grace), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assertCan(t, tt.want, engine, tt.actor, tt.action, tt.resource)
		})
	}
}

func TestEngine_NoPoliciesDeniesEverything(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	removeSeededPolicies(t, db)
	ada := insertIdentity(t, db)

	assertCan(t, false, newEngine(db), ada, testAction, identityResource(ada))
}

func TestEngine_ActionGroups(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	removeSeededPolicies(t, db)
	addPolicy(t, db, `permit (principal, action in Action::"read", resource);`)
	engine := newEngine(db)
	ada := insertIdentity(t, db)
	grace := insertIdentity(t, db)

	assertCan(t, true, engine, ada, testAction, identityResource(grace))
	assertCan(t, false, engine, ada, NewAction("identity:delete"), identityResource(grace))
}

func TestEngine_ForbidOnActionGroupOverridesPermit(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	removeSeededPolicies(t, db)
	addPolicy(t, db, `
		permit (principal, action, resource);
		forbid (principal, action in Action::"read", resource);
	`)
	ada := insertIdentity(t, db)

	assertCan(t, false, newEngine(db), ada, testAction, identityResource(ada))
}

func TestEngine_GroupsOfPrincipal(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	member := insertIdentity(t, db)
	outsider := insertIdentity(t, db)
	readers := insertGroup(t, db, member)
	removeSeededPolicies(t, db)
	addPolicy(t, db, `permit (principal in Group::"`+readers.String()+`", action, resource);`)
	engine := newEngine(db)

	assertCan(t, true, engine, member, testAction, identityResource(outsider))
	assertCan(t, true, engine, member, NewAction("other:anything"), Resource{Type: "Other", ID: outsider})
	assertCan(t, false, engine, outsider, testAction, identityResource(member))
}

func TestEngine_GroupsOfResource(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	ada := insertIdentity(t, db)
	grace := insertIdentity(t, db)
	visible := insertGroup(t, db)
	secret := insertGroup(t, db)
	removeSeededPolicies(t, db)
	addPolicy(t, db, `
		permit (principal, action, resource in Group::"`+visible.String()+`");
		forbid (principal, action, resource in Group::"`+secret.String()+`");
	`)
	engine := newEngine(db)

	assertCan(t, true, engine, ada, testAction, identityResource(grace, visible))
	assertCan(t, false, engine, ada, testAction, identityResource(grace))
	assertCan(t, false, engine, ada, testAction, identityResource(grace, visible, secret))
}

// TestEngine_PrincipalInGroupResource checks that a group as the resource is the entity that its members
// are in, so that a policy can let the members of a group read it.
func TestEngine_PrincipalInGroupResource(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	member := insertIdentity(t, db)
	outsider := insertIdentity(t, db)
	maintainers := insertGroup(t, db, member)
	removeSeededPolicies(t, db)
	addPolicy(t, db, `permit (principal, action, resource is Group) when { principal in resource };`)
	engine := newEngine(db)
	resource := Resource{Type: GroupType, ID: maintainers}

	assertCan(t, true, engine, member, testAction, resource)
	assertCan(t, false, engine, outsider, testAction, resource)
}

// ownedResource returns a resource of an owned type that belongs to the identity with the given groups.
func ownedResource(owner uuid.UUID, ownerGroups ...uuid.UUID) Resource {
	return Resource{Type: "APIKey", ID: uuid.NewV4(), Owner: &Owner{ID: owner, Groups: ownerGroups}}
}

// TestEngine_Owner checks that policies can refer to the owner of a resource, and to the owner's groups,
// so that a forbid on them works.
func TestEngine_Owner(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	ada := insertIdentity(t, db)
	grace := insertIdentity(t, db)
	carol := insertIdentity(t, db)
	secret := insertGroup(t, db, grace)
	removeSeededPolicies(t, db)
	addPolicy(t, db, `
		permit (principal, action, resource) when { resource has owner && resource.owner == principal };
		permit (principal == Identity::"`+ada.String()+`", action, resource is APIKey);
		forbid (principal, action, resource is APIKey) when { resource.owner in Group::"`+secret.String()+`" };
	`)
	engine := newEngine(db)

	assertCan(t, true, engine, carol, testAction, ownedResource(carol))
	assertCan(t, false, engine, carol, testAction, ownedResource(ada))
	assertCan(t, true, engine, ada, testAction, ownedResource(carol))
	assertCan(t, false, engine, ada, testAction, ownedResource(grace, secret))
	assertCan(t, false, engine, grace, testAction, ownedResource(grace, secret))
	// Without its groups, the owner doesn't match the forbid, which is why a resource must carry them.
	assertCan(t, true, engine, ada, testAction, ownedResource(grace))
}

// TestEngine_MissingOwner checks that a policy on the owner of a resource that lacks one fails the decision
// instead of skipping the policy.
func TestEngine_MissingOwner(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	ada := insertIdentity(t, db)
	removeSeededPolicies(t, db)
	addPolicy(t, db, `
		permit (principal, action, resource);
		forbid (principal, action, resource is APIKey) when { resource.owner == principal };
	`)

	decider, err := newEngine(db).For(t.Context(), ada)
	require.NoError(t, err)

	can, err := decider.Can(testAction, Resource{Type: "APIKey", ID: uuid.NewV4()})

	assert.False(t, can)
	assert.Error(t, err)
}

// failingCondition is a condition that fails to evaluate, since the entity lacks the attribute.
const failingCondition = `when { resource.missing == principal };`

// TestEngine_FailingForbid checks that a forbid that fails to evaluate fails the decision, since Cedar skips
// it, so that the permit alone would decide.
func TestEngine_FailingForbid(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	removeSeededPolicies(t, db)
	policy := addPolicy(t, db, `
		permit (principal, action, resource);
		forbid (principal, action, resource) `+failingCondition)
	ada := insertIdentity(t, db)

	decider, err := newEngine(db).For(t.Context(), ada)
	require.NoError(t, err)

	can, err := decider.Can(testAction, identityResource(ada))

	assert.False(t, can, "a forbid that fails to evaluate must never result in access")
	require.Error(t, err)
	assert.Contains(t, err.Error(), policy.String()+"#1", "the error must name the policy and the statement")
}

// TestEngine_CountsDecisions checks that every decision is counted by its result only: allow, deny, or error
// for a forbid that fails to evaluate.
func TestEngine_CountsDecisions(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	removeSeededPolicies(t, db)
	ada, bob := insertIdentity(t, db), insertIdentity(t, db)
	addPolicy(t, db, `
		permit (principal, action, resource);
		forbid (principal, action, resource) when { principal == resource };
		forbid (principal, action, resource is `+GroupType+`) `+failingCondition)
	metrics := telemetrytest.New(t)

	decider, err := New(db, metrics.MeterProvider(), tracenoop.NewTracerProvider()).For(t.Context(), ada)
	require.NoError(t, err)
	for range 2 {
		can, err := decider.Can(testAction, identityResource(bob))
		require.NoError(t, err)
		require.True(t, can)
	}
	can, err := decider.Can(testAction, identityResource(ada))
	require.NoError(t, err)
	require.False(t, can)
	_, err = decider.Can(testAction, Resource{Type: GroupType, ID: uuid.NewV4()})
	require.Error(t, err)

	got := map[string]int64{}
	for _, point := range metrics.SumPoints[int64](t, "pkgsilo.authz.decisions") {
		require.Equal(t, 1, point.Attributes.Len(), "the decision is the only attribute")
		decision, _ := point.Attributes.Value("pkgsilo.authz.decision")
		got[decision.AsString()] = point.Value
	}
	assert.Equal(t, map[string]int64{"allow": 2, "deny": 1, "error": 1}, got)
}

// TestEngine_FailingPermit checks that a permit that fails to evaluate grants nothing, without failing the
// decision, so that the other statements still decide.
func TestEngine_FailingPermit(t *testing.T) {
	t.Parallel()
	const failingPermit = `permit (principal, action, resource) ` + failingCondition

	tests := []struct {
		name    string
		source  string
		wantCan bool
	}{
		{name: "alone", source: failingPermit, wantCan: false},
		{name: "beside a permit", source: `permit (principal, action, resource);` + failingPermit, wantCan: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			db := databasetest.New(t)
			removeSeededPolicies(t, db)
			addPolicy(t, db, tt.source)
			ada := insertIdentity(t, db)

			assertCan(t, tt.wantCan, newEngine(db), ada, testAction, identityResource(ada))
		})
	}
}

// TestEngine_Revision checks that the policies are parsed once per revision and that a new revision
// applies on the next request of every instance. A policy that was stored without a new revision shows
// which policies an instance uses.
func TestEngine_Revision(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	ada := insertIdentity(t, db)
	grace := insertIdentity(t, db)
	meterProvider, tracerProvider := noop.NewMeterProvider(), tracenoop.NewTracerProvider()
	first, second := New(db, meterProvider, tracerProvider), New(db, meterProvider, tracerProvider)
	assertCan(t, false, first, ada, testAction, identityResource(grace))
	assertCan(t, false, second, ada, testAction, identityResource(grace))

	insertPolicyRow(t, db, `permit (principal, action, resource);`)
	assertCan(t, false, first, ada, testAction, identityResource(grace))
	assertCan(t, false, second, ada, testAction, identityResource(grace))

	bumpRevision(t, db)
	assertCan(t, true, first, ada, testAction, identityResource(grace))
	assertCan(t, true, second, ada, testAction, identityResource(grace))

	// A removal applies the same way, so that access is revoked at once.
	_, err := db.ExecContext(t.Context(), `DELETE FROM policies WHERE source = 'permit (principal, action, resource);'`)
	require.NoError(t, err)
	bumpRevision(t, db)
	assertCan(t, false, first, ada, testAction, identityResource(grace))
	assertCan(t, false, second, ada, testAction, identityResource(grace))
}

func TestEngine_GroupsAreReadPerRequest(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	engine := newEngine(db)
	ada := insertIdentity(t, db)
	grace := insertIdentity(t, db)
	readers := insertGroup(t, db)
	addPolicy(t, db, `permit (principal in Group::"`+readers.String()+`", action, resource);`)
	assertCan(t, false, engine, ada, testAction, identityResource(grace))

	addMembers(t, db, readers, ada)

	assertCan(t, true, engine, ada, testAction, identityResource(grace))
}

func TestEngine_UnparsablePolicy(t *testing.T) {
	t.Parallel()
	db := databasetest.New(t)
	policy := addPolicy(t, db, `permit (`)

	_, err := newEngine(db).For(t.Context(), uuid.NewV4())

	require.Error(t, err)
	assert.Contains(t, err.Error(), policy.String(), "the error must name the policy")
}

func TestEngine_DatabaseFails(t *testing.T) {
	t.Parallel()
	for _, table := range []string{"policy_revision", "policies", "group_members"} {
		t.Run(table, func(t *testing.T) {
			t.Parallel()
			db := databasetest.New(t)
			_, err := db.ExecContext(t.Context(), `DROP TABLE `+table)
			require.NoError(t, err)

			_, err = newEngine(db).For(t.Context(), uuid.NewV4())

			assert.Error(t, err)
		})
	}
}
