package authz

import (
	"errors"
	"fmt"
	"strings"
)

// ErrDenied is returned when the actor may read an object but not do the action to it. An object that the
// actor may not read is reported like a missing one instead, so that it stays hidden.
var ErrDenied = errors.New("access denied")

// FieldsDeniedError reports the fields of a write that the actor may not set. It matches ErrDenied.
type FieldsDeniedError struct {
	// Fields holds the names of the denied fields, in the order of their declaration.
	Fields []string
}

func (e *FieldsDeniedError) Error() string {
	return "access denied to fields " + strings.Join(e.Fields, ", ")
}

// Is reports whether target is ErrDenied, so that code that doesn't tell the fields apart still denies.
func (*FieldsDeniedError) Is(target error) bool {
	return target == ErrDenied
}

// FieldActions are the actions of setting each field of an object in one kind of write, such as
// identity:update:username, as members of the action group of the write, such as identity:update.
type FieldActions struct {
	fields  []string
	actions []Action
}

// NewFieldActions declares the actions <group>:<field> for the given fields, as members of the action group
// group, which is named <type>:<verb>.
func NewFieldActions(group string, fields ...string) FieldActions {
	actions := make([]Action, len(fields))
	for i, field := range fields {
		actions[i] = NewAction(group+":"+field, group)
	}

	return FieldActions{fields: fields, actions: actions}
}

// Fields returns the names of the fields, in the order of their declaration.
func (f FieldActions) Fields() []string {
	return append([]string(nil), f.fields...)
}

// Check checks the action of setting every field on the resource, as a write that sets them all needs. It
// checks all of them, so that a denial lists every denied field, and returns a *FieldsDeniedError if any
// is denied.
func (f FieldActions) Check(decider Decider, resource Resource) error {
	var denied []string
	for i, action := range f.actions {
		can, err := decider.Can(action, resource)
		if err != nil {
			return fmt.Errorf("check fields: %w", err)
		}
		if !can {
			denied = append(denied, f.fields[i])
		}
	}

	if len(denied) > 0 {
		return &FieldsDeniedError{Fields: denied}
	}

	return nil
}
