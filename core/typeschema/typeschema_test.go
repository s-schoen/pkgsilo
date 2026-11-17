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
// which only an envelope.Field leaves optional.
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
				"type": ["null", "array"],
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

type (
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

// TestFor_Invalid checks that For fails for the wrappers in the wrong direction or without omitzero.
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := typeschema.For(tt.t, tt.dir)

			assert.Error(t, err)
		})
	}
}
