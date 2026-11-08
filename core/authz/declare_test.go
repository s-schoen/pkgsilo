package authz

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testModel struct {
	ID        string
	Name      Field[string]    `authz:"name"`
	OwnerID   Field[string]    `authz:"ownerId"`
	CreatedAt Field[time.Time] `authz:"createdAt"`
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

	got := DeclareType(TypeSpec{
		Type:    "APIKey",
		Owned:   true,
		Model:   reflect.TypeFor[testModel](),
		Create:  reflect.TypeFor[testInput](),
		Update:  reflect.TypeFor[testInput](),
		Patch:   reflect.TypeFor[testPatch](),
		Actions: []Action{extra},
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
		Actions: []Action{got.Read(), got.Delete(), extra},
		Fields:  []FieldActions{got.ReadFields(), got.CreateFields(), got.UpdateFields()},
	}, got.Declaration())
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

func TestSetFields(t *testing.T) {
	t.Parallel()
	name := "Ada"
	tests := []struct {
		name  string
		input any
		want  []string
	}{
		{name: "patch of nothing", input: testPatch{}, want: nil},
		{name: "patch of some", input: testPatch{OwnerID: &name}, want: []string{"ownerId"}},
		{name: "patch of all", input: testPatch{Name: &name, OwnerID: &name}, want: []string{"name", "ownerId"}},
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
