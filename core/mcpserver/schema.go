package mcpserver

import (
	"reflect"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"

	"pkgsilo/core/typeschema"
)

// schemaFor returns the JSON schema of the arguments (output false) or of the output (output true) of a
// tool, t: the schema of typeschema.For, in which an envelope.Field of the output is optional and has the
// schema of its value.
//
// No member of the arguments allows null, even a pointer or a slice, which Registry.Add rejects (see
// arguments.decode).
func schemaFor(t reflect.Type, output bool) (*jsonschema.Schema, error) {
	dir := typeschema.Input
	if output {
		dir = typeschema.Output
	}
	schema, err := typeschema.For(t, dir)
	if err != nil {
		return nil, err //nolint:wrapcheck // Names t.
	}
	if !output {
		for _, member := range schema.Properties {
			disallowNull(member)
		}
	}

	return schema, nil
}

// disallowNull removes null from the types that schema allows, such as those of a pointer, ["null", "string"].
func disallowNull(schema *jsonschema.Schema) {
	if !slices.Contains(schema.Types, "null") {
		return
	}

	schema.Types = slices.DeleteFunc(slices.Clone(schema.Types), func(t string) bool { return t == "null" })
	if len(schema.Types) == 1 {
		schema.Type, schema.Types = schema.Types[0], nil
	}
}
