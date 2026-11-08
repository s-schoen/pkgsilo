package authz

import (
	"errors"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testReadFields = NewReadFieldActions("identity", "username", "displayName")

func TestNewReadFieldActions(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"username", "displayName"}, testReadFields.Fields())
	assert.Equal(t, []Action{
		NewAction("identity:read:username", "read", "identity:readFields"),
		NewAction("identity:read:displayName", "read", "identity:readFields"),
	}, testReadFields.Actions())
}

// TestNewReadFieldActions_Schema checks that the read field actions are declared with both of their action
// groups, so that policies can grant them with `action in Action::"read"` and
// `action in Action::"identity:readFields"`.
func TestNewReadFieldActions_Schema(t *testing.T) {
	t.Parallel()

	got := Schema(Declaration{Type: IdentityType, Fields: []FieldActions{testReadFields}})

	const appliesTo = ` appliesTo { principal: Identity, resource: Identity, context: {} };`
	assert.Equal(t, []string{
		`entity Group;`,
		`entity Identity in [Group];`,
		`action "identity:readFields";`,
		`action "read";`,
		`action "identity:read:username" in ["identity:readFields", "read"]` + appliesTo,
		`action "identity:read:displayName" in ["identity:readFields", "read"]` + appliesTo,
		``,
	}, strings.Split(got, "\n"))
}

func TestFieldActions_Readable(t *testing.T) {
	t.Parallel()
	resource := Resource{Type: IdentityType, ID: uuid.NewV4()}
	tests := []struct {
		name         string
		allowed      []string
		wantUsername Field[string]
		wantDisplay  Field[string]
	}{
		{
			name: "all", allowed: []string{"username", "displayName"},
			wantUsername: Visible("ada"), wantDisplay: Visible("Ada Lovelace"),
		},
		{name: "one", allowed: []string{"displayName"}, wantDisplay: Visible("Ada Lovelace")},
		{name: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var checked []Action
			decider := deciderFunc(func(action Action, got Resource) (bool, error) {
				assert.Equal(t, resource, got)
				checked = append(checked, action)
				for _, field := range tt.allowed {
					if assert.ObjectsAreEqual(NewAction("identity:read:"+field, "read", "identity:readFields"), action) {
						return true, nil
					}
				}

				return false, nil
			})

			fields, err := testReadFields.Readable(decider, resource)
			require.NoError(t, err)

			assert.Equal(t, testReadFields.Actions(), checked)
			assert.Equal(t, tt.wantUsername, Reveal(fields, "username", "ada"))
			assert.Equal(t, tt.wantDisplay, Reveal(fields, "displayName", "Ada Lovelace"))
		})
	}
}

func TestFieldActions_Readable_DecisionFails(t *testing.T) {
	t.Parallel()
	errDecision := errors.New("policy failed")
	decider := deciderFunc(func(Action, Resource) (bool, error) { return false, errDecision })

	_, err := testReadFields.Readable(decider, Resource{Type: IdentityType, ID: uuid.NewV4()})

	require.ErrorIs(t, err, errDecision)
}

func TestFieldActions_ReadableIf(t *testing.T) {
	t.Parallel()
	resource := Resource{Type: IdentityType, ID: uuid.NewV4()}
	readObject := NewAction("identity:read", ActionGroupRead)
	for _, canRead := range []bool{true, false} {
		t.Run(map[bool]string{true: "readable", false: "unreadable"}[canRead], func(t *testing.T) {
			t.Parallel()
			var checked []Action
			decider := deciderFunc(func(action Action, got Resource) (bool, error) {
				assert.Equal(t, resource, got)
				checked = append(checked, action)
				if assert.ObjectsAreEqual(readObject, action) {
					return canRead, nil
				}

				return true, nil
			})

			fields, err := testReadFields.ReadableIf(decider, readObject, resource)
			require.NoError(t, err)

			if canRead {
				assert.Equal(t, append([]Action{readObject}, testReadFields.Actions()...), checked)
				assert.Equal(t, Visible("ada"), Reveal(fields, "username", "ada"))
				return
			}
			assert.Equal(t, []Action{readObject}, checked, "no field of an unreadable object must be checked")
			assert.Equal(t, Field[string]{}, Reveal(fields, "username", "ada"))
		})
	}
}

func TestFieldActions_ReadableIf_DecisionFails(t *testing.T) {
	t.Parallel()
	errDecision := errors.New("policy failed")
	decider := deciderFunc(func(Action, Resource) (bool, error) { return false, errDecision })

	_, err := testReadFields.ReadableIf(decider, NewAction("identity:read"), Resource{Type: IdentityType})

	require.ErrorIs(t, err, errDecision)
}

func TestFieldActions_NoneReadable(t *testing.T) {
	t.Parallel()
	fields := testReadFields.NoneReadable()

	assert.Equal(t, Field[string]{}, Reveal(fields, "username", "ada"))
	assert.Equal(t, Field[string]{}, Reveal(fields, "displayName", "Ada Lovelace"))
}

// TestReveal_UndeclaredField checks that a field without a read action is a bug that fails loudly, instead
// of staying hidden from every actor.
func TestReveal_UndeclaredField(t *testing.T) {
	t.Parallel()
	fields := testReadFields.NoneReadable()

	assert.PanicsWithValue(t, `authz: no read action declared for the field "email"`, func() {
		Reveal(fields, "email", "ada@example.com")
	})
}

func TestField(t *testing.T) {
	t.Parallel()

	value, visible := Visible("ada").Get()
	assert.Equal(t, "ada", value)
	assert.True(t, visible)

	value, visible = Field[string]{}.Get()
	assert.Empty(t, value)
	assert.False(t, visible, "the zero field must be hidden")
}
