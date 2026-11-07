package authz

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchema(t *testing.T) {
	t.Parallel()

	got := Schema(
		Declaration{
			Type:    IdentityType,
			Actions: []Action{NewAction("identity:read", ActionGroupRead), NewAction("identity:delete")},
			Fields:  []FieldActions{NewFieldActions("identity:update", "name")},
		},
		Declaration{
			Type:    "Policy",
			Actions: []Action{NewAction("policy:read", ActionGroupRead, "policy:all")},
		},
		// Declared twice, by declarations that each add actions to it.
		Declaration{Type: "Policy", Actions: []Action{NewAction("policy:delete", "policy:all")}},
		// The base types aren't declared again.
		Declaration{Type: "Group"},
	)

	const appliesToIdentity = ` appliesTo { principal: Identity, resource: Identity, context: {} };`
	const appliesToPolicy = ` appliesTo { principal: Identity, resource: Policy, context: {} };`
	assert.Equal(t, []string{
		`entity Group;`,
		`entity Identity in [Group];`,
		`entity Policy;`,
		`action "read";`,
		`action "identity:update";`,
		`action "policy:all";`,
		`action "identity:read" in ["read"]` + appliesToIdentity,
		`action "identity:delete"` + appliesToIdentity,
		`action "identity:update:name" in ["identity:update"]` + appliesToIdentity,
		`action "policy:read" in ["policy:all", "read"]` + appliesToPolicy,
		`action "policy:delete" in ["policy:all"]` + appliesToPolicy,
		``,
	}, strings.Split(got, "\n"))
}

// TestSchema_ActionAsGroup checks that an action group that is a declared action too is declared once.
func TestSchema_ActionAsGroup(t *testing.T) {
	t.Parallel()

	got := Schema(Declaration{
		Type:    IdentityType,
		Actions: []Action{NewAction("identity:all"), NewAction("identity:read", "identity:all")},
	})

	assert.Equal(t, 1, strings.Count(got, `action "identity:all"`), got)
}

func TestSchema_None(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "entity Group;\nentity Identity in [Group];\n", Schema())
}
