package authz

import (
	"errors"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testFields = NewFieldActions("identity:update", "username", "displayName")

func TestNewFieldActions(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"username", "displayName"}, testFields.Fields())
	assert.Equal(t, []Action{
		NewAction("identity:update:username", "identity:update"),
		NewAction("identity:update:displayName", "identity:update"),
	}, testFields.actions)
}

func TestFieldActions_Fields_ReturnsCopy(t *testing.T) {
	t.Parallel()

	testFields.Fields()[0] = "changed"

	assert.Equal(t, []string{"username", "displayName"}, testFields.Fields())
}

func TestFieldActions_Check(t *testing.T) {
	t.Parallel()
	resource := Resource{Type: IdentityType, ID: uuid.NewV4()}
	tests := []struct {
		name       string
		allowed    []string
		wantDenied []string
	}{
		{name: "all allowed", allowed: []string{"username", "displayName"}},
		{name: "one denied", allowed: []string{"displayName"}, wantDenied: []string{"username"}},
		{name: "all denied", wantDenied: []string{"username", "displayName"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var checked []Action
			decider := deciderFunc(func(action Action, got Resource) (bool, error) {
				assert.Equal(t, resource, got)
				checked = append(checked, action)
				for _, field := range tt.allowed {
					if assert.ObjectsAreEqual(NewAction("identity:update:"+field, "identity:update"), action) {
						return true, nil
					}
				}

				return false, nil
			})

			err := testFields.Check(decider, resource)

			assert.Equal(t, testFields.actions, checked, "every field must be checked, so that all denials are listed")
			if tt.wantDenied == nil {
				require.NoError(t, err)
				return
			}
			var denied *FieldsDeniedError
			require.ErrorAs(t, err, &denied)
			assert.Equal(t, tt.wantDenied, denied.Fields)
			assert.ErrorIs(t, err, ErrDenied)
		})
	}
}

func TestFieldActions_Check_DecisionFails(t *testing.T) {
	t.Parallel()
	errDecision := errors.New("policy failed")
	decider := deciderFunc(func(Action, Resource) (bool, error) { return false, errDecision })

	err := testFields.Check(decider, Resource{Type: IdentityType, ID: uuid.NewV4()})

	require.ErrorIs(t, err, errDecision)
	assert.NotErrorIs(t, err, ErrDenied, "a failed decision must not look like a denial")
}

func TestFieldsDeniedError(t *testing.T) {
	t.Parallel()
	err := &FieldsDeniedError{Fields: []string{"username", "displayName"}}

	require.EqualError(t, err, "access denied to fields username, displayName")
	assert.ErrorIs(t, err, ErrDenied)
}

func TestFieldActions_Only(t *testing.T) {
	t.Parallel()
	resource := Resource{Type: IdentityType, ID: uuid.NewV4()}
	tests := []struct {
		name   string
		fields []string
		want   []Action
	}{
		{name: "none", fields: nil, want: nil},
		{name: "one", fields: []string{"displayName"}, want: []Action{testFields.actions[1]}},
		// In the order of their declaration, so that a denial lists them like Check does.
		{name: "all, in another order", fields: []string{"displayName", "username"}, want: testFields.actions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var checked []Action
			decider := deciderFunc(func(action Action, got Resource) (bool, error) {
				assert.Equal(t, resource, got)
				checked = append(checked, action)

				return false, nil
			})

			err := testFields.Only(tt.fields...).Check(decider, resource)

			assert.Equal(t, tt.want, checked)
			if tt.want == nil {
				require.NoError(t, err)
				return
			}
			var denied *FieldsDeniedError
			require.ErrorAs(t, err, &denied)
			assert.Equal(t, testFields.Only(tt.fields...).Fields(), denied.Fields)
		})
	}
}

// TestFieldActions_Only_UndeclaredField checks that a field without an action is a bug that fails loudly,
// instead of being written unchecked.
func TestFieldActions_Only_UndeclaredField(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, `authz: no action declared for the field "email"`, func() {
		testFields.Only("username", "email")
	})
}
