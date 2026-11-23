package typeschema_test

import (
	"encoding/json/v2"
	"net/netip"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
	"pkgsilo/core/typeschema"
)

// item is a member of the output of output.
type item struct {
	Label netip.Addr                  `json:"label"`
	Alias envelope.Field[*netip.Addr] `json:"alias,omitzero"`
	When  envelope.Field[*time.Time]  `json:"when,omitzero"`
}

type output struct {
	Items      []item               `json:"items"`
	NextOffset *int                 `json:"nextOffset,omitempty"`
	Owners     map[string]uuid.UUID `json:"owners"`
	Skipped    string               `json:"-"`
}

// TestFor_Output checks the schema of an output with fields, UUIDs, times and text inside slices and maps,
// which only an envelope.Field leaves optional, and whose slices aren't null, since encoding/json/v2 encodes
// a nil slice as empty.
func TestFor_Output(t *testing.T) {
	t.Parallel()

	schema, err := typeschema.For(reflect.TypeFor[output](), typeschema.Output)

	require.NoError(t, err)
	b, err := json.Marshal(schema)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {
			"items": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"label": {"type": "string"},
						"alias": {"type": ["null", "string"]},
						"when": {"type": ["null", "string"], "format": "date-time"}
					},
					"required": ["label"],
					"additionalProperties": false
				}
			},
			"nextOffset": {"type": ["null", "integer"]},
			"owners": {"type": "object", "additionalProperties": {"type": "string", "format": "uuid"}}
		},
		"required": ["items", "owners"],
		"additionalProperties": false
	}`, string(b))
}

type patch struct {
	Name    request.Optional[string]    `json:"name,omitzero"    jsonschema:"The name."`
	Expires request.Optional[time.Time] `json:"expires,omitzero"`
	Owner   uuid.UUID                   `json:"owner"`
}

// TestFor_Input checks the schema of an input with optional members, which have the schema of their value.
func TestFor_Input(t *testing.T) {
	t.Parallel()

	schema, err := typeschema.For(reflect.TypeFor[patch](), typeschema.Input)

	require.NoError(t, err)
	b, err := json.Marshal(schema)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "The name."},
			"expires": {"type": "string", "format": "date-time"},
			"owner": {"type": "string", "format": "uuid"}
		},
		"required": ["owner"],
		"additionalProperties": false
	}`, string(b))
}

// action is a string type with a fixed set of values.
type action string

func (action) EnumValues() []string {
	return []string{"thing:delete", "thing:archive"}
}

type withActions struct {
	Actions envelope.Field[[]action] `json:"actions,omitzero" jsonschema:"The actions."`
}

// TestFor_Enum checks that a type that implements Enum is a string with its values, also inside a field and
// a slice.
func TestFor_Enum(t *testing.T) {
	t.Parallel()

	schema, err := typeschema.For(reflect.TypeFor[withActions](), typeschema.Output)

	require.NoError(t, err)
	b, err := json.Marshal(schema)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type": "object",
		"properties": {
			"actions": {
				"type": "array",
				"items": {"type": "string", "enum": ["thing:delete", "thing:archive"]},
				"description": "The actions."
			}
		},
		"additionalProperties": false
	}`, string(b))
}

// intEnum implements Enum, but isn't a string.
type intEnum int

func (intEnum) EnumValues() []string {
	return []string{"one"}
}

type (
	enumNotString struct {
		Value intEnum `json:"value"`
	}
	fieldWithoutOmitZero struct {
		Name envelope.Field[string] `json:"name"`
	}
	optionalWithoutOmitZero struct {
		Name request.Optional[string] `json:"name"`
	}
	fieldInSlice struct {
		Names []envelope.Field[string] `json:"names"`
	}
	optionalInStruct struct {
		Patch patch `json:"patch"`
	}
)

// TestFor_Invalid checks that For fails for the wrappers in the wrong direction or without omitzero, and for
// an Enum that isn't a string.
func TestFor_Invalid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		t    reflect.Type
		dir  typeschema.Direction
	}{
		{name: "field without omitzero", t: reflect.TypeFor[fieldWithoutOmitZero](), dir: typeschema.Output},
		{name: "optional without omitzero", t: reflect.TypeFor[optionalWithoutOmitZero](), dir: typeschema.Input},
		{name: "field in input", t: reflect.TypeFor[fieldInSlice](), dir: typeschema.Input},
		{name: "optional in output", t: reflect.TypeFor[optionalInStruct](), dir: typeschema.Output},
		{name: "enum not a string", t: reflect.TypeFor[enumNotString](), dir: typeschema.Output},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := typeschema.For(tt.t, tt.dir)

			assert.Error(t, err)
		})
	}
}
