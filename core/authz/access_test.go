package authz

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accessActions are the actions of a type with every kind of action: fields to read, create and update, and
// further actions, one of which reads.
var accessActions = DeclareType(TypeSpec{
	Type:    "Thing",
	Model:   reflect.TypeFor[testModel](),
	Create:  reflect.TypeFor[testInput](),
	Update:  reflect.TypeFor[testInput](),
	Actions: []Action{NewAction("thing:archive"), NewAction("thing:readHistory", ActionGroupRead)},
})

// purposeDecider allows the actions with the given names, and records the names that it was asked about,
// by Can and by Hint apart.
type purposeDecider struct {
	allowed []string
	// fail names the action whose decision fails.
	fail        string
	can, hinted []string
}

func (d *purposeDecider) Can(action Action, _ Resource) (bool, error) {
	d.can = append(d.can, action.Name())

	return d.decide(action)
}

func (d *purposeDecider) Hint(action Action, _ Resource) (bool, error) {
	d.hinted = append(d.hinted, action.Name())

	return d.decide(action)
}

// CanSome panics: Access decides about its object.
func (*purposeDecider) CanSome(action Action, _ string) (bool, error) {
	panic("unexpected CanSome " + action.Name())
}

func (d *purposeDecider) decide(action Action) (bool, error) {
	if action.Name() == d.fail {
		return false, errDecision
	}

	return slices.Contains(d.allowed, action.Name()), nil
}

var (
	errDecision    = errors.New("decision failed")
	accessResource = Resource{Type: "Thing", ID: uuid.NewV4()}
)

// TestTypeActions_Access checks that Access enforces the field reads and hints at the allowed actions, and
// that the allowed actions hold no reads of the object or its fields and no creates, but further reads.
func TestTypeActions_Access(t *testing.T) {
	t.Parallel()
	decider := &purposeDecider{allowed: []string{
		"thing:read:name", "thing:delete", "thing:update:ownerId", "thing:readHistory",
		// Never part of the allowed actions, even when allowed.
		"thing:read", "thing:create:name",
	}}

	access, err := accessActions.Access(decider, accessResource)

	require.NoError(t, err)
	assert.Equal(t, AllowedActions{"thing:delete", "thing:update:ownerId", "thing:readHistory"},
		access.AllowedActions())
	assert.Equal(t, Visible("Ada"), access.Fields().Reveal("name", "Ada"))
	assert.Equal(t, Field[string]{}, access.Fields().Reveal("ownerId", "o1"))
	assert.Equal(t, []string{"thing:read:name", "thing:read:ownerId", "thing:read:createdAt"}, decider.can,
		"the readable fields must be enforced")
	assert.Equal(t, accessActions.Allowable(), decider.hinted, "the allowed actions must be hints")
}

// TestTypeActions_Access_NothingAllowed checks that an object that the actor may do nothing to has an empty
// list of allowed actions, not nil, unlike one that it may not read.
func TestTypeActions_Access_NothingAllowed(t *testing.T) {
	t.Parallel()

	access, err := accessActions.Access(&purposeDecider{}, accessResource)

	require.NoError(t, err)
	assert.NotNil(t, access.AllowedActions())
	assert.Empty(t, access.AllowedActions())
}

func TestTypeActions_Access_Error(t *testing.T) {
	t.Parallel()
	for _, fail := range []string{"thing:read:ownerId", "thing:update:name"} {
		t.Run(fail, func(t *testing.T) {
			t.Parallel()

			access, err := accessActions.Access(&purposeDecider{allowed: []string{"thing:delete"}, fail: fail},
				accessResource)

			require.ErrorIs(t, err, errDecision)
			assert.Equal(t, Access{}, access)
		})
	}
}

// TestTypeActions_AccessIf checks that AccessIf checks the read of the object first, and gives an object
// that the actor may not read neither fields nor actions, without asking about them.
func TestTypeActions_AccessIf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		allowed     []string
		wantAllowed AllowedActions
		wantName    Field[string]
		wantChecked int
	}{
		{
			name:        "readable",
			allowed:     []string{"thing:read", "thing:read:name", "thing:delete"},
			wantAllowed: AllowedActions{"thing:delete"}, wantName: Visible("Ada"),
			wantChecked: 1 + len(accessActions.ReadFields().Fields()),
		},
		{name: "not readable", allowed: []string{"thing:read:name", "thing:delete"}, wantChecked: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			decider := &purposeDecider{allowed: tt.allowed}

			access, err := accessActions.AccessIf(decider, accessResource)

			require.NoError(t, err)
			assert.Equal(t, tt.wantAllowed, access.AllowedActions())
			assert.Equal(t, tt.wantName, access.Fields().Reveal("name", "Ada"))
			assert.Equal(t, "thing:read", decider.can[0], "the read must be checked first")
			assert.Len(t, decider.can, tt.wantChecked)
		})
	}
}

func TestTypeActions_AccessIf_Error(t *testing.T) {
	t.Parallel()

	_, err := accessActions.AccessIf(&purposeDecider{fail: "thing:read"}, accessResource)

	require.ErrorIs(t, err, errDecision)
}

func TestTypeActions_NoAccess(t *testing.T) {
	t.Parallel()

	access := accessActions.NoAccess()

	assert.Nil(t, access.AllowedActions())
	assert.Equal(t, accessActions.ReadFields().NoneReadable(), access.Fields())
}
