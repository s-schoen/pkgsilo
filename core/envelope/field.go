package envelope

import (
	"encoding/json"
	"fmt"
)

// Field is a member of a resource representation that the caller may not be allowed to see. A hidden
// field is absent from the response, while a visible one is its value, even if that's null. Use it with
// the omitzero option, such as `json:"expiresAt,omitzero"`: without it, a hidden field would be null.
//
// The zero Field is hidden. Its members have no JSON names of their own, since MarshalJSON encodes the value
// alone.
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

// IsZero reports whether the field is hidden, so that omitzero leaves it out.
func (f Field[T]) IsZero() bool {
	return !f.Visible
}

// MarshalJSON encodes the field's value.
func (f Field[T]) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(f.Value)
	if err != nil {
		return nil, fmt.Errorf("encode field: %w", err)
	}

	return b, nil
}

// UnmarshalJSON decodes a member that's present as a visible field. A member that's absent leaves the
// field hidden.
func (f *Field[T]) UnmarshalJSON(data []byte) error {
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("decode field: %w", err)
	}
	*f = Field[T]{Value: value, Visible: true}

	return nil
}
