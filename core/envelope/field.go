package envelope

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
)

// errHiddenFieldEncoded is the error of a hidden field that's encoded instead of left out.
var errHiddenFieldEncoded = errors.New("envelope: hidden field encoded outside a Resource or Collection")

// Field is a member of a resource representation that the caller may not be allowed to see. A hidden
// field is absent from the response, while a visible one is its value, even if that's null. Resource and
// Collection leave hidden fields out, so a member needs no omitzero option. Every member of their
// representations but the id is a Field, even one that's never hidden: they leave out every zero member,
// and a Field is zero only while it's hidden. Encoding a hidden field any other way fails, instead of
// answering with null.
//
// The zero Field is hidden. Its members have no JSON names of their own, since MarshalJSONTo encodes the
// value alone.
type Field[T any] struct {
	Value   T    `json:"-"`
	Visible bool `json:"-"`
}

// NewField returns the field with the given value, visible or hidden, such as from the Get method of a
// field of the domain model.
func NewField[T any](value T, visible bool) Field[T] {
	if !visible {
		return Field[T]{}
	}

	return Field[T]{Value: value, Visible: true}
}

// Visible returns the field with the given value, visible.
func Visible[T any](value T) Field[T] {
	return Field[T]{Value: value, Visible: true}
}

func (Field[T]) isField() {}

// ValueType returns T, the type of the field's value, for code that describes a representation by its Go
// type, such as the output schema of an MCP tool, which is that of T.
func (Field[T]) ValueType() reflect.Type {
	return reflect.TypeFor[T]()
}

// IsZero reports whether the field is hidden, so that Resource and Collection leave it out.
func (f Field[T]) IsZero() bool {
	return !f.Visible
}

// MarshalJSONTo encodes the field's value. The value keeps its zero members, even inside a Resource or
// Collection, which leave out only the zero members of the body itself.
func (f Field[T]) MarshalJSONTo(enc *jsontext.Encoder) error {
	if !f.Visible {
		return errHiddenFieldEncoded
	}
	if err := json.MarshalEncode(enc, f.Value, json.OmitZeroStructFields(false)); err != nil {
		return fmt.Errorf("encode field: %w", err)
	}

	return nil
}

// UnmarshalJSONFrom decodes a member that's present as a visible field, even if it's null. A member that's
// absent leaves the field hidden.
func (f *Field[T]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	var value T
	if err := json.UnmarshalDecode(dec, &value); err != nil {
		return err //nolint:wrapcheck // The json package adds the member's pointer to its own error.
	}
	*f = Field[T]{Value: value, Visible: true}

	return nil
}
