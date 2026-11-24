package authz

import (
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testModel struct {
	ID        string
	Name      Field[string]    `authz:"name"`
	OwnerID   Field[string]    `authz:"ownerId"`
	CreatedAt Field[time.Time] `authz:"createdAt"`

	AllowedActions AllowedActions
}

type testInput struct {
	Name    string `authz:"name"`
	OwnerID string `authz:"ownerId"`
}

type testPatch struct {
	Name    *string `authz:"name"`
	OwnerID *string `authz:"ownerId"`
}

func TestDeclareType(t *testing.T) {
	t.Parallel()
	extra := NewAction("apikey:rotate")
	readExtra := NewAction("apikey:readUsage", ActionGroupRead)
	collection := NewAction("apikey:readStats", ActionGroupRead)

	got := DeclareType(TypeSpec{
		Type:              "APIKey",
		Owned:             true,
		Model:             reflect.TypeFor[testModel](),
		Create:            reflect.TypeFor[testInput](),
		Update:            reflect.TypeFor[testInput](),
		Patch:             reflect.TypeFor[testPatch](),
		Actions:           []Action{extra, readExtra},
		CollectionActions: []Action{collection},
	})

	assert.Equal(t, "APIKey", got.Type())
	assert.Equal(t, NewAction("apikey:read", ActionGroupRead), got.Read())
	assert.Equal(t, NewAction("apikey:delete"), got.Delete())
	assert.Equal(t, NewReadFieldActions("apikey", "name", "ownerId", "createdAt"), got.ReadFields())
	assert.Equal(t, NewFieldActions("apikey:create", "name", "ownerId"), got.CreateFields())
	assert.Equal(t, NewFieldActions("apikey:update", "name", "ownerId"), got.UpdateFields())
	assert.Equal(t, Declaration{
		Type:    "APIKey",
		Owned:   true,
		Actions: []Action{got.Read(), got.Delete(), extra, readExtra, collection},
		Fields:  []FieldActions{got.ReadFields(), got.CreateFields(), got.UpdateFields()},
	}, got.Declaration())
	assert.Equal(t,
		[]string{"apikey:delete", "apikey:update:name", "apikey:update:ownerId", "apikey:rotate", "apikey:readUsage"},
		got.Allowable(), "no read of the object or a field, no create and no action on the collection")
	assert.Equal(t, []string{
		"apikey:read", "apikey:delete", "apikey:rotate", "apikey:readUsage", "apikey:readStats",
		"apikey:read:name", "apikey:read:ownerId", "apikey:read:createdAt",
		"apikey:create:name", "apikey:create:ownerId",
		"apikey:update:name", "apikey:update:ownerId",
	}, actionNames(got.Actions()), "every action, but no action group")
	assert.True(t, got.Owned())
	assert.False(t, got.OnePerOwner())
}

// TestTypeActions_OnePerOwner checks that a type tells whether it's OnePerOwner and Owned.
func TestTypeActions_OnePerOwner(t *testing.T) {
	t.Parallel()
	model := reflect.TypeFor[testModel]()

	password := DeclareType(TypeSpec{Type: "Password", Owned: true, OnePerOwner: true, Model: model})
	policy := DeclareType(TypeSpec{Type: "Policy", Model: model})

	assert.True(t, password.Owned())
	assert.True(t, password.OnePerOwner())
	assert.False(t, policy.Owned())
	assert.False(t, policy.OnePerOwner())
}

// actionNames returns the names of the actions, in their order.
func actionNames(actions []Action) []string {
	names := make([]string, len(actions))
	for i, action := range actions {
		names[i] = action.Name()
	}

	return names
}

// TestDeclareType_WithoutWrites checks that a type without Create and Update has no field actions for
// writes, and that its schema has none.
func TestDeclareType_WithoutWrites(t *testing.T) {
	t.Parallel()

	got := DeclareType(TypeSpec{Type: "Policy", Model: reflect.TypeFor[testModel]()})

	assert.Empty(t, got.CreateFields().Fields())
	assert.Empty(t, got.UpdateFields().Fields())
	schema := Schema(got.Declaration())
	assert.NotContains(t, schema, "policy:create")
	assert.NotContains(t, schema, "policy:update")
	assert.Contains(t, schema, `action "policy:read:ownerId" in ["policy:readFields", "read"]`)
	assert.Equal(t, []string{"policy:delete"}, got.Allowable())
}

// TestDeclareType_Invalid checks that a spec whose types would leave a field unchecked, or that would name
// an action wrongly, panics, so that the feature's package fails to initialize.
func TestDeclareType_Invalid(t *testing.T) {
	t.Parallel()
	model := reflect.TypeFor[testModel]()
	input := reflect.TypeFor[testInput]()
	tests := []struct {
		name string
		spec TypeSpec
		want string
	}{
		{
			name: "no model",
			spec: TypeSpec{Type: "Policy"},
			want: "no struct type",
		},
		{
			name: "model member that's no Field",
			spec: TypeSpec{Type: "Policy", Model: reflect.TypeFor[struct {
				ID   string
				Name string `authz:"name"`
			}]()},
			want: "not an authz.Field",
		},
		{
			name: "model member without a tag",
			spec: TypeSpec{Type: "Policy", Model: reflect.TypeFor[struct {
				ID   string
				Name Field[string]
			}]()},
			want: "member Name",
		},
		{
			name: "model without AllowedActions",
			spec: TypeSpec{Type: "Policy", Model: reflect.TypeFor[struct {
				ID   string
				Name Field[string] `authz:"name"`
			}]()},
			want: "has no member AllowedActions of type authz.AllowedActions",
		},
		{
			name: "AllowedActions of another type",
			spec: TypeSpec{Type: "Policy", Model: reflect.TypeFor[struct {
				ID             string
				AllowedActions []string
			}]()},
			want: "member AllowedActions",
		},
		{
			name: "AllowedActions under another name",
			spec: TypeSpec{Type: "Policy", Model: reflect.TypeFor[struct {
				ID      string
				Actions AllowedActions
			}]()},
			want: "member Actions",
		},
		{
			name: "OnePerOwner without Owned",
			spec: TypeSpec{Type: "Password", OnePerOwner: true, Model: model},
			want: "OnePerOwner but not Owned",
		},
		{
			name: "tag with a colon",
			spec: TypeSpec{Type: "Policy", Model: model, Create: reflect.TypeFor[struct {
				Name string `authz:"na:me"`
			}]()},
			want: `"na:me"`,
		},
		{
			name: "tag in upper camel case",
			spec: TypeSpec{Type: "Policy", Model: model, Update: reflect.TypeFor[struct {
				Name string `authz:"Name"`
			}]()},
			want: `"Name"`,
		},
		{
			name: "duplicate tag",
			spec: TypeSpec{Type: "Policy", Model: model, Create: reflect.TypeFor[struct {
				Name  string `authz:"name"`
				Title string `authz:"name"`
			}]()},
			want: "more than one member",
		},
		{
			name: "input that's no struct",
			spec: TypeSpec{Type: "Policy", Model: model, Create: reflect.TypeFor[*testInput]()},
			want: "no struct type",
		},
		{
			name: "patch without update",
			spec: TypeSpec{Type: "Policy", Model: model, Create: input, Patch: reflect.TypeFor[testPatch]()},
			want: "without an update",
		},
		{
			name: "patch member that's no pointer",
			spec: TypeSpec{Type: "Policy", Model: model, Update: input, Patch: input},
			want: "not a pointer",
		},
		{
			name: "patch with other fields",
			spec: TypeSpec{Type: "Policy", Model: model, Update: input, Patch: reflect.TypeFor[struct {
				Name *string `authz:"name"`
			}]()},
			want: "aren't the fields",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				msg, _ := recover().(string)
				assert.Contains(t, msg, tt.want)
			}()

			DeclareType(tt.spec)
		})
	}
}

// TestTypeActions_NewResource checks that a new resource of a OnePerOwner type is the owner's one resource,
// and that of another owned type has a fresh ID of its own, both with the owner.
func TestTypeActions_NewResource(t *testing.T) {
	t.Parallel()
	model := reflect.TypeFor[testModel]()
	owner := &Owner{ID: uuid.NewV4(), Groups: []uuid.UUID{uuid.NewV4()}}

	t.Run("one per owner", func(t *testing.T) {
		t.Parallel()
		actions := DeclareType(TypeSpec{Type: "Password", Owned: true, OnePerOwner: true, Model: model})

		assert.Equal(t, Resource{Type: "Password", ID: owner.ID, Owner: owner}, actions.NewResource(owner))
	})

	t.Run("many per owner", func(t *testing.T) {
		t.Parallel()
		actions := DeclareType(TypeSpec{Type: "APIKey", Owned: true, Model: model})

		first, second := actions.NewResource(owner), actions.NewResource(owner)

		assert.Equal(t, "APIKey", first.Type)
		assert.Same(t, owner, first.Owner)
		assert.NotEqual(t, uuid.UUID{}, first.ID)
		assert.NotEqual(t, owner.ID, first.ID, "not the owner's ID")
		assert.NotEqual(t, first.ID, second.ID, "a fresh ID each time")
		assert.Empty(t, first.Groups)
	})

	t.Run("not owned", func(t *testing.T) {
		t.Parallel()
		actions := DeclareType(TypeSpec{Type: "Policy", Model: model})

		require.PanicsWithValue(t, "authz: type Policy isn't Owned, so its resources have no owner", func() {
			actions.NewResource(owner)
		})
	})
}

func TestSetFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input any
		want  []string
	}{
		{name: "patch of nothing", input: testPatch{}, want: nil},
		{name: "patch of some", input: testPatch{OwnerID: new("Ada")}, want: []string{"ownerId"}},
		{name: "patch of all", input: testPatch{Name: new("Ada"), OwnerID: new("Ada")}, want: []string{"name", "ownerId"}},
		// A member that's no pointer is always set, even to its zero value.
		{name: "input", input: testInput{}, want: []string{"name", "ownerId"}},
		{name: "input with an optional member", input: struct {
			Name      string     `authz:"name"`
			ExpiresAt *time.Time `authz:"expiresAt"`
		}{}, want: []string{"name"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, SetFields(tt.input))
		})
	}
}

// TestSetFields_Untagged checks that a member without a tag panics, since it would be written unchecked.
func TestSetFields_Untagged(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t,
		"authz: member Name of struct { Name *string } has the field name \"\", not a tag `authz:\"<field>\"` "+
			"with a name in lower camel case",
		func() { SetFields(struct{ Name *string }{}) })
}
