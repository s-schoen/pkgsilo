package mcpserver

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/jsonschema-go/jsonschema"
)

// The errors of the members of a tool's arguments that don't match its input schema, which
// InvalidArgumentsError names next to the errors of the input's Validate.
var (
	errArgumentMissing   = validation.NewError("validation_required", "is required")
	errArgumentNull      = validation.NewError("validation_not_null", "cannot be null")
	errArgumentUnknown   = validation.NewError("validation_unknown", "is not an argument of this tool")
	errArgumentDuplicate = validation.NewError("validation_duplicate", "appears more than once")
	errArgumentRange     = validation.NewError("validation_range", "is out of range")
	errArgumentInvalid   = validation.NewError("validation_invalid", "is invalid")
)

// errArgumentsNotObject answers arguments that aren't a JSON object, whose members are the arguments.
var errArgumentsNotObject = DefineError(invalidArgumentsCode, "The arguments must be a JSON object.")

// arguments decodes the arguments of a tool into its input, In, and checks them against its input schema,
// so that every mismatch is answered with invalid_arguments, naming the member and why, as the errors of
// In's Validate are, rather than with an error of the SDK or the json package.
type arguments struct {
	schema *jsonschema.Schema
	// expected names the value that each member takes, such as "a UUID", for the error of a value that
	// doesn't decode into its Go type.
	expected map[string]string
}

// newArguments returns the arguments of a tool whose input schema, as schemaFor builds it, is schema.
func newArguments(schema *jsonschema.Schema) *arguments {
	expected := make(map[string]string, len(schema.Properties))
	for name, member := range schema.Properties {
		expected[name] = expectedValue(member)
	}

	return &arguments{schema: schema, expected: expected}
}

// expectedValue names the value of the schema, such as "a UUID" or "an integer".
func expectedValue(schema *jsonschema.Schema) string {
	if schema.Format == "uuid" {
		return "a UUID"
	}

	typ := schema.Type
	if typ == "" {
		// Such as ["null", "integer"], though no input schema allows null.
		if i := slices.IndexFunc(schema.Types, func(t string) bool { return t != "null" }); i >= 0 {
			typ = schema.Types[i]
		}
	}
	switch typ {
	case "":
		return "a JSON value"
	case "integer", "object", "array":
		return "an " + typ
	default:
		return "a " + typ
	}
}

// decode decodes raw, the arguments of a call, into in, which must be a pointer to a struct. It answers
// arguments that aren't an object with errArgumentsNotObject, and with invalid_arguments a member that's
// unknown, null or present twice, a required member that's missing, and a value that doesn't decode into
// the member's Go type, such as a malformed UUID. Arguments that are absent or null are an empty object.
func (a *arguments) decode(raw []byte, in any) *ToolError {
	if len(raw) == 0 {
		raw = []byte("{}")
	}

	var members map[string]jsontext.Value
	if err := json.Unmarshal(raw, &members); err != nil {
		if name, ok := duplicateName(err); ok {
			return InvalidArgumentsError(validation.Errors{name: errArgumentDuplicate})
		}
		return errArgumentsNotObject
	}
	if errs := a.checkMembers(members); len(errs) > 0 {
		return InvalidArgumentsError(errs)
	}

	if err := json.Unmarshal(raw, in); err != nil {
		return a.valueError(err)
	}

	return nil
}

// checkMembers returns the errors of the members of the arguments, by their names: a member that's unknown
// or null, and a required member that's missing.
func (a *arguments) checkMembers(members map[string]jsontext.Value) validation.Errors {
	errs := validation.Errors{}
	for name, value := range members {
		switch {
		case a.schema.Properties[name] == nil:
			errs[name] = errArgumentUnknown
		case value.Kind() == 'n':
			errs[name] = errArgumentNull
		}
	}
	for _, name := range a.schema.Required {
		if _, ok := members[name]; !ok {
			errs[name] = errArgumentMissing
		}
	}

	return errs
}

// duplicateName returns the name of the member that err, of unmarshaling an object, reports twice.
func duplicateName(err error) (string, bool) {
	syntaxErr, ok := errors.AsType[*jsontext.SyntacticError](err)
	if !ok || !errors.Is(err, jsontext.ErrDuplicateName) {
		return "", false
	}

	return memberName(syntaxErr.JSONPointer), true
}

// valueError returns the invalid_arguments error that answers err, of decoding a value that doesn't decode
// into the Go type of its member: a member of another type, such as a string for an integer, a value that
// the Go type rejects, such as a malformed UUID, or an integer that overflows it.
func (a *arguments) valueError(err error) *ToolError {
	semanticErr, ok := errors.AsType[*json.SemanticError](err)
	if !ok || semanticErr.JSONPointer == "" {
		// The members were checked, so this is no mismatch of a member, which arguments that are an object
		// can't have otherwise.
		return errArgumentsNotObject
	}

	name := memberName(semanticErr.JSONPointer)
	switch expected, ok := a.expected[name]; {
	case errors.Is(err, strconv.ErrRange):
		return InvalidArgumentsError(validation.Errors{name: errArgumentRange})
	case ok:
		return InvalidArgumentsError(validation.Errors{name: validation.NewError("validation_type", "must be "+expected)})
	default:
		// A value inside a member, such as an item of an array.
		return InvalidArgumentsError(validation.Errors{name: errArgumentInvalid})
	}
}

// memberName names the member that ptr points to by its path, with its names joined by dots, such as
// "address.city", as the HTTP API names a member of a body.
func memberName(ptr jsontext.Pointer) string {
	return strings.Join(slices.Collect(ptr.Tokens()), ".")
}

// validateArguments validates in and returns the invalid_arguments error if it's invalid. It panics if the
// validation fails for another reason, such as a misconfigured rule: a bug in the feature, not in the
// arguments.
func validateArguments(in validation.Validatable) *ToolError {
	err := in.Validate()
	if err == nil {
		return nil
	}
	errs, ok := errors.AsType[validation.Errors](err)
	if !ok {
		panic(fmt.Errorf("mcpserver: validate arguments: %w", err))
	}

	return InvalidArgumentsError(errs)
}
