package request

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// Optional is a member of a request body that a client may leave out, such as in a JSON Merge Patch
// (RFC 7396), where a member that's absent stays unchanged. It tells the three states of a member apart:
// absent, null and a value.
type Optional[T any] struct {
	// Value is the member's value. It's the zero value if the member is absent or null.
	Value T
	// Set reports whether the body has the member, even as null.
	Set bool
	// Null reports whether the member is null.
	Null bool
}

// UnmarshalJSONFrom decodes the member's value. The json package calls it only for a member that's present.
//
// It decodes the value with the options of dec, so that a value inside it rejects unknown members as the
// body does, and returns the json package's error as it is, which names the member by its JSON Pointer.
func (o *Optional[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	o.Set = true
	if dec.PeekKind() == 'n' {
		if _, err := dec.ReadToken(); err != nil {
			return err //nolint:wrapcheck // A syntactic error, which the json package locates itself.
		}
		o.Null = true

		return nil
	}

	return json.UnmarshalDecode(dec, &o.Value) //nolint:wrapcheck // The json package adds the member's pointer.
}

// Pointer returns a pointer to the value, or nil if the member is absent or null. It suits a member that
// IfSet validated, which can't be null.
func (o Optional[T]) Pointer() *T {
	if !o.Set || o.Null {
		return nil
	}

	return &o.Value
}

// optional is an Optional of any type, for IfSet, whose rule is called with the Optional as any.
type optional interface {
	state() (value any, set, null bool)
}

func (o Optional[T]) state() (value any, set, null bool) {
	return o.Value, o.Set, o.Null
}

// errNull reports a null member that must have a value if it's present.
var errNull = validation.NewError("validation_not_null", "cannot be null")

// IfSet returns a rule for an Optional member that may be absent but not null, such as a required field in
// a JSON Merge Patch: it skips the member if it's absent, rejects it if it's null and applies rules to its
// value otherwise. Like the rules of a plain member, it reports an error under the member's name.
//
// Applied to a value that isn't an Optional, it fails with a validation.InternalError: a misconfigured
// rule, which DecodeJSON and DecodeMergePatch panic on.
func IfSet(rules ...validation.Rule) validation.Rule {
	return validation.By(func(v any) error {
		o, ok := v.(optional)
		if !ok {
			return validation.NewInternalError(errors.New("request: IfSet applied to a value that is not an Optional"))
		}

		value, set, null := o.state()
		switch {
		case !set:
			return nil
		case null:
			return errNull
		default:
			return validation.Validate(value, rules...)
		}
	})
}
