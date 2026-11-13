package authz

import (
	"fmt"
	"slices"
)

// Field is a field of an object as an actor sees it: its value if the actor may read it, and nothing
// otherwise. The zero Field is hidden, so that a field that a service forgets to reveal stays hidden.
type Field[T any] struct {
	// Value is the field's value, which is the zero value of T if the field is hidden.
	Value T
	// Visible reports whether the actor may read the field.
	Visible bool
}

// Visible returns the field with the given value, visible.
func Visible[T any](value T) Field[T] {
	return Field[T]{Value: value, Visible: true}
}

// Get returns the field's value and whether it's visible, the arguments of a constructor such as
// envelope.NewField.
func (f Field[T]) Get() (T, bool) {
	return f.Value, f.Visible
}

// NewReadFieldActions declares the actions <type>:read:<field> of reading each field of an object of the
// given type, such as identity:read:username, as members of the action groups read and <type>:readFields.
// They come on top of the action <type>:read of reading the object, which is checked first and reveals
// only its ID.
func NewReadFieldActions(objectType string, fields ...string) FieldActions {
	actions := make([]Action, len(fields))
	for i, field := range fields {
		actions[i] = NewAction(objectType+":read:"+field, ActionGroupRead, objectType+":readFields")
	}

	return FieldActions{fields: fields, actions: actions}
}

// ReadableFields are the fields of one object that an actor may read, out of the fields of FieldActions.
type ReadableFields struct {
	fields   []string
	readable []bool
}

// Readable checks the action of reading every field on the resource, and returns the fields that the
// decider allows reading. The resource must be one that the decider allows reading at all.
func (f FieldActions) Readable(decider Decider, resource Resource) (ReadableFields, error) {
	readable := make([]bool, len(f.actions))
	for i, action := range f.actions {
		can, err := decider.Can(action, resource)
		if err != nil {
			return ReadableFields{}, fmt.Errorf("check readable fields: %w", err)
		}
		readable[i] = can
	}

	return ReadableFields{fields: f.fields, readable: readable}, nil
}

// ReadableIf checks the action read of reading the object first, and returns the fields of the resource
// that the decider allows reading like Readable if it allows that, and no field otherwise. It's for an
// object whose read isn't checked yet, such as one that the actor creates, which it may not be allowed to
// read.
func (f FieldActions) ReadableIf(decider Decider, read Action, resource Resource) (ReadableFields, error) {
	can, err := decider.Can(read, resource)
	if err != nil {
		return ReadableFields{}, fmt.Errorf("check read: %w", err)
	}
	if !can {
		return f.NoneReadable(), nil
	}

	return f.Readable(decider, resource)
}

// NoneReadable returns that no field is readable, for an object that the actor may not read at all, such as
// one that it created without being allowed to read it.
func (f FieldActions) NoneReadable() ReadableFields {
	return ReadableFields{fields: f.fields, readable: make([]bool, len(f.fields))}
}

// Reveal returns the field with the given name and value, visible if it's readable and hidden otherwise.
// It panics on a field that isn't declared, which is a bug in the caller: the field would stay hidden
// from every actor.
func (f ReadableFields) Reveal[T any](name string, value T) Field[T] {
	i := slices.Index(f.fields, name)
	if i < 0 {
		panic(fmt.Sprintf("authz: no read action declared for the field %q", name))
	}
	if !f.readable[i] {
		return Field[T]{}
	}

	return Visible(value)
}
