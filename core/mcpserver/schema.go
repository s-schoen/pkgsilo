package mcpserver

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"uuid"

	"github.com/google/jsonschema-go/jsonschema"

	"pkgsilo/core/envelope"
)

// fieldValue is implemented by envelope.Field, which reports the type of its value.
type fieldValue interface {
	ValueType() reflect.Type
}

var (
	fieldPkgPath    = reflect.TypeFor[envelope.Field[any]]().PkgPath()
	fieldValueType  = reflect.TypeFor[fieldValue]()
	textMarshaler   = reflect.TypeFor[encoding.TextMarshaler]()
	uuidType        = reflect.TypeFor[uuid.UUID]()
	errFieldNotZero = errors.New("an envelope.Field must be tagged omitzero, so that a hidden field is left out")
	errFieldInInput = errors.New("an envelope.Field is for output only")
)

// schemaFor returns the JSON schema of the arguments (output false) or of the output (output true) of a
// tool, t. It's the schema that the SDK infers from the json tags with jsonschema.ForType, except for the
// types that encode as something other than their Go kind:
//
//   - an envelope.Field[T] has the schema of T, which allows null if T is a pointer. The member that holds
//     it must be tagged omitzero, so that it's optional, since a hidden field is absent. Only the output
//     may hold one.
//   - a uuid.UUID, an array of 16 bytes, is a string in the uuid format.
//   - any other type that implements encoding.TextMarshaler is a string, as encoding/json encodes it.
//
// No member of the arguments allows null, even a pointer or a slice, which Registry.Add rejects (see
// arguments.decode).
func schemaFor(t reflect.Type, output bool) (*jsonschema.Schema, error) {
	overrides := map[reflect.Type]*jsonschema.Schema{}
	if err := collectOverrides(t, output, overrides, map[reflect.Type]bool{}); err != nil {
		return nil, err
	}

	schema, err := jsonschema.ForType(t, &jsonschema.ForOptions{TypeSchemas: overrides})
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

// collectOverrides adds the schemas of the types inside t that schemaFor describes otherwise than
// jsonschema.ForType to overrides. seen holds the types already visited.
func collectOverrides(
	t reflect.Type, output bool, overrides map[reflect.Type]*jsonschema.Schema, seen map[reflect.Type]bool,
) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if seen[t] {
		return nil
	}
	seen[t] = true

	switch {
	case isField(t):
		if !output {
			return fmt.Errorf("%s: %w", t, errFieldInInput)
		}
		return fieldOverride(t, overrides, seen)
	case t == uuidType:
		overrides[t] = &jsonschema.Schema{Type: "string", Format: "uuid"}
	case isText(t):
		overrides[t] = &jsonschema.Schema{Type: "string"}
	case t.Kind() == reflect.Struct:
		return structOverrides(t, output, overrides, seen)
	case slices.Contains([]reflect.Kind{reflect.Slice, reflect.Array, reflect.Map}, t.Kind()):
		return collectOverrides(t.Elem(), output, overrides, seen)
	}

	return nil
}

// isText reports whether t, or a pointer to it, encodes as a JSON string with encoding.TextMarshaler.
func isText(t reflect.Type) bool {
	return t.Implements(textMarshaler) || reflect.PointerTo(t).Implements(textMarshaler)
}

// structOverrides is collectOverrides for the struct t, whose members holding an envelope.Field must be
// tagged omitzero.
func structOverrides(
	t reflect.Type, output bool, overrides map[reflect.Type]*jsonschema.Schema, seen map[reflect.Type]bool,
) error {
	for _, field := range reflect.VisibleFields(t) {
		tag := field.Tag.Get("json")
		if !field.IsExported() || field.Anonymous || tag == "-" {
			continue
		}
		_, options, _ := strings.Cut(tag, ",")
		if isField(field.Type) && !strings.Contains(","+options+",", ",omitzero,") {
			return fmt.Errorf("member %s of %s: %w", field.Name, t, errFieldNotZero)
		}
		if err := collectOverrides(field.Type, output, overrides, seen); err != nil {
			return err
		}
	}

	return nil
}

// fieldOverride adds the schema of the envelope.Field t, that of its value, to overrides.
func fieldOverride(t reflect.Type, overrides map[reflect.Type]*jsonschema.Schema, seen map[reflect.Type]bool) error {
	valueType := reflect.Zero(t).Interface().(fieldValue).ValueType() //nolint:forcetypeassert // isField checked it.
	if err := collectOverrides(valueType, true, overrides, seen); err != nil {
		return err
	}
	schema, err := jsonschema.ForType(valueType, &jsonschema.ForOptions{TypeSchemas: overrides})
	if err != nil {
		return err //nolint:wrapcheck // Names the type.
	}
	overrides[t] = schema

	return nil
}

// isField reports whether t is an envelope.Field.
func isField(t reflect.Type) bool {
	return t.PkgPath() == fieldPkgPath && strings.HasPrefix(t.Name(), "Field[") && t.Implements(fieldValueType)
}
