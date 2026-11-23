// Package typeschema derives the JSON Schema (draft 2020-12) of a Go type from its json tags, as pkgsilo
// encodes and decodes it with encoding/json/v2. The input and output schemas of the MCP tools and the
// schemas of the OpenAPI document of the HTTP API come from it, so that both describe the same types alike.
package typeschema

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/google/jsonschema-go/jsonschema"

	"pkgsilo/core/envelope"
	"pkgsilo/core/request"
)

// Direction tells whether a type is read from a request or written to a response, which decides the
// wrapper types that it may hold.
type Direction int

const (
	// Input is the direction of a request body or tool's arguments, which may hold a request.Optional.
	Input Direction = iota
	// Output is the direction of a response body or tool's output, which may hold an envelope.Field.
	Output
)

// jsonString is the schema type of a JSON string.
const jsonString = "string"

// Enum is implemented by a string type whose values are a fixed set, such as the actions that a resource's
// allowedActions can hold: its schema is a string with these values as its enum.
type Enum interface {
	// EnumValues returns the values of the type, the same for every value of it.
	EnumValues() []string
}

// valueTyper is implemented by envelope.Field and request.Optional, which report the type of their value.
type valueTyper interface {
	ValueType() reflect.Type
}

var (
	fieldPkgPath    = reflect.TypeFor[envelope.Field[any]]().PkgPath()
	optionalPkgPath = reflect.TypeFor[request.Optional[any]]().PkgPath()
	valueTyperType  = reflect.TypeFor[valueTyper]()
	enumType        = reflect.TypeFor[Enum]()
	textMarshaler   = reflect.TypeFor[encoding.TextMarshaler]()
	uuidType        = reflect.TypeFor[uuid.UUID]()
	timeType        = reflect.TypeFor[time.Time]()

	errNotOmitZero      = errors.New("must be tagged omitzero, so that it's optional")
	errFieldInInput     = errors.New("an envelope.Field is for output only")
	errOptionalInOutput = errors.New("a request.Optional is for input only")
	errEnumNotString    = errors.New("an Enum must be a string type")
)

// For returns the JSON schema of t in the direction dir. It's the schema that jsonschema.ForType infers
// from the json tags, in which a member is required unless it's tagged omitempty or omitzero, and its
// jsonschema tag is its description, except for the types that encode as something other than their Go
// kind:
//
//   - an envelope.Field[T] has the schema of T, which allows null if T is a pointer. Only the output may
//     hold one, and its member must be tagged omitzero, since a hidden field is absent.
//   - a request.Optional[T] has the schema of T. Only the input may hold one, and its member must be tagged
//     omitzero, since it may be absent.
//   - a type that implements Enum is a string with its EnumValues as the enum.
//   - a slice or a map of the output is never null, since encoding/json/v2 encodes a nil one as empty.
//   - a uuid.UUID, an array of 16 bytes, is a string in the uuid format.
//   - a time.Time is a string in the date-time format.
//   - any other type that implements encoding.TextMarshaler is a string, as encoding/json encodes it.
//
// It fails for a type that the rules above or jsonschema.ForType reject, a bug of the caller.
func For(t reflect.Type, dir Direction) (*jsonschema.Schema, error) {
	overrides := map[reflect.Type]*jsonschema.Schema{}
	if err := collectOverrides(t, dir, overrides, map[reflect.Type]bool{}); err != nil {
		return nil, err
	}

	schema, err := jsonschema.ForType(t, &jsonschema.ForOptions{TypeSchemas: overrides})
	if err != nil {
		return nil, err //nolint:wrapcheck // Names t.
	}

	return schema, nil
}

// collectOverrides adds the schemas of the types inside t that For describes otherwise than
// jsonschema.ForType to overrides. seen holds the types already visited.
func collectOverrides(
	t reflect.Type, dir Direction, overrides map[reflect.Type]*jsonschema.Schema, seen map[reflect.Type]bool,
) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if seen[t] {
		return nil
	}
	seen[t] = true

	switch {
	case isWrapped(t):
		return valueOverride(t, dir, overrides, seen)
	case t.Implements(enumType):
		if t.Kind() != reflect.String {
			return fmt.Errorf("%s: %w", t, errEnumNotString)
		}
		overrides[t] = enumSchema(t)
	case textSchema(t) != nil:
		overrides[t] = textSchema(t)
	case t.Kind() == reflect.Struct:
		return structOverrides(t, dir, overrides, seen)
	case slices.Contains([]reflect.Kind{reflect.Slice, reflect.Array, reflect.Map}, t.Kind()):
		return containerOverrides(t, dir, overrides, seen)
	}

	return nil
}

// enumSchema returns the schema of t, a string type that implements Enum: a string with its values as the enum.
func enumSchema(t reflect.Type) *jsonschema.Schema {
	values := reflect.Zero(t).Interface().(Enum).EnumValues() //nolint:forcetypeassert // The caller checked it.
	enum := make([]any, len(values))
	for i, value := range values {
		enum[i] = value
	}

	return &jsonschema.Schema{Type: jsonString, Enum: enum}
}

// textSchema returns the schema of t if it encodes as a JSON string with encoding.TextMarshaler, or a pointer
// to it does, and nil otherwise.
func textSchema(t reflect.Type) *jsonschema.Schema {
	switch {
	case t == uuidType:
		return &jsonschema.Schema{Type: jsonString, Format: "uuid"}
	case t == timeType:
		return &jsonschema.Schema{Type: jsonString, Format: "date-time"}
	case t.Implements(textMarshaler) || reflect.PointerTo(t).Implements(textMarshaler):
		return &jsonschema.Schema{Type: jsonString}
	default:
		return nil
	}
}

// structOverrides is collectOverrides for the struct t, whose members holding an envelope.Field or a
// request.Optional must be tagged omitzero.
func structOverrides(
	t reflect.Type, dir Direction, overrides map[reflect.Type]*jsonschema.Schema, seen map[reflect.Type]bool,
) error {
	for _, field := range reflect.VisibleFields(t) {
		tag := field.Tag.Get("json")
		if !field.IsExported() || field.Anonymous || tag == "-" {
			continue
		}
		_, options, _ := strings.Cut(tag, ",")
		if isWrapped(field.Type) && !strings.Contains(","+options+",", ",omitzero,") {
			return fmt.Errorf("member %s of %s: %w", field.Name, t, errNotOmitZero)
		}
		if err := collectOverrides(field.Type, dir, overrides, seen); err != nil {
			return err
		}
	}

	return nil
}

// valueOverride adds the schema of the wrapper t, an envelope.Field or a request.Optional, to overrides: that
// of its value. It fails for an envelope.Field of the input and a request.Optional of the output.
func valueOverride(
	t reflect.Type, dir Direction, overrides map[reflect.Type]*jsonschema.Schema, seen map[reflect.Type]bool,
) error {
	switch {
	case isWrapper(t, fieldPkgPath, "Field[") && dir != Output:
		return fmt.Errorf("%s: %w", t, errFieldInInput)
	case isWrapper(t, optionalPkgPath, "Optional[") && dir != Input:
		return fmt.Errorf("%s: %w", t, errOptionalInOutput)
	}
	valueType := reflect.Zero(t).Interface().(valueTyper).ValueType() //nolint:forcetypeassert // isWrapper checked it.
	if err := collectOverrides(valueType, dir, overrides, seen); err != nil {
		return err
	}
	schema, err := jsonschema.ForType(valueType, &jsonschema.ForOptions{TypeSchemas: overrides})
	if err != nil {
		return err //nolint:wrapcheck // Names the type.
	}
	overrides[t] = schema

	return nil
}

// containerOverrides is collectOverrides for the slice, array or map t. A slice or map of the output gets the
// schema of jsonschema.ForType without null, since encoding/json/v2 encodes a nil one as empty.
func containerOverrides(
	t reflect.Type, dir Direction, overrides map[reflect.Type]*jsonschema.Schema, seen map[reflect.Type]bool,
) error {
	if err := collectOverrides(t.Elem(), dir, overrides, seen); err != nil {
		return err
	}
	if t.Kind() == reflect.Array || dir != Output {
		return nil
	}

	schema, err := jsonschema.ForType(t, &jsonschema.ForOptions{TypeSchemas: overrides})
	if err != nil {
		return err //nolint:wrapcheck // Names the type.
	}
	types := slices.DeleteFunc(slices.Clone(schema.Types), func(t string) bool { return t == "null" })
	if len(types) == 1 {
		schema.Type, schema.Types = types[0], nil
	}
	overrides[t] = schema

	return nil
}

// isWrapped reports whether t is an envelope.Field or a request.Optional.
func isWrapped(t reflect.Type) bool {
	return isWrapper(t, fieldPkgPath, "Field[") || isWrapper(t, optionalPkgPath, "Optional[")
}

// isWrapper reports whether t is the generic type of pkgPath whose name starts with prefix and that reports
// the type of its value.
func isWrapper(t reflect.Type, pkgPath, prefix string) bool {
	return t.PkgPath() == pkgPath && strings.HasPrefix(t.Name(), prefix) && t.Implements(valueTyperType)
}
