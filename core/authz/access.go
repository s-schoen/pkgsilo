package authz

import (
	"fmt"
	"slices"
)

// AllowedActions holds the names of the actions that an actor may do to one object, such as identity:delete
// and identity:update:displayName, in the order of TypeActions.Allowable, so that a client can offer only
// what's allowed. It's a member of every model, named AllowedActions.
//
// It holds no read actions of the object or its fields, which the object and its visible fields already
// tell, and no create actions, which no existing object has. It's advisory: a client may still be denied,
// such as when the policies change in between, and the service checks every request on its own.
type AllowedActions []string

// Access is what an actor may do to one object: the fields of its model that the actor may read, and the
// actions that it may do to it. A Mapper builds the model from it.
type Access struct {
	fields  ReadableFields
	allowed AllowedActions
}

// Fields returns the fields that the actor may read, for a model built by hand with ReadableFields.Reveal.
func (a Access) Fields() ReadableFields {
	return a.fields
}

// AllowedActions returns the names of the actions that the actor may do, for a model built by hand.
func (a Access) AllowedActions() AllowedActions {
	return slices.Clone(a.allowed)
}

// FieldsOnly returns the access with the readable fields and no allowed actions. It only bridges the services
// that don't fill AllowedActions yet, and goes once every service uses TypeActions.Access.
func FieldsOnly(fields ReadableFields) Access {
	return Access{fields: fields}
}

// Access checks what the decider allows on the resource: reading each field of the Model, and each action
// of Allowable, which count as hints. The resource must be one that the decider allows reading at all. The
// allowed actions are never nil, so that an object that the actor may do nothing to tells so.
func (a TypeActions) Access(decider Decider, resource Resource) (Access, error) {
	fields, err := a.readFields.Readable(decider, resource)
	if err != nil {
		return Access{}, err
	}

	allowed := AllowedActions{}
	for _, action := range a.allowable {
		can, err := decider.Hint(action, resource)
		if err != nil {
			return Access{}, fmt.Errorf("check allowed actions: %w", err)
		}
		if can {
			allowed = append(allowed, action.Name())
		}
	}

	return Access{fields: fields, allowed: allowed}, nil
}

// AccessIf checks the action Read of reading the object first, and returns what the decider allows on the
// resource like Access if it allows that, and NoAccess otherwise. It's for an object whose read isn't
// checked yet, such as one that the actor creates, which it may not be allowed to read.
func (a TypeActions) AccessIf(decider Decider, resource Resource) (Access, error) {
	can, err := decider.Can(a.read, resource)
	if err != nil {
		return Access{}, fmt.Errorf("check read: %w", err)
	}
	if !can {
		return a.NoAccess(), nil
	}

	return a.Access(decider, resource)
}

// NoAccess returns that no field is readable and no action allowed, for an object that the actor may not
// read at all, such as one that it created without being allowed to read it. Its allowed actions are nil,
// unlike those of an object that the actor may read but do nothing to.
func (a TypeActions) NoAccess() Access {
	return Access{fields: a.readFields.NoneReadable()}
}

// Allowable returns the names of the actions that AllowedActions can hold, in its order: <type>:delete,
// <type>:update:<field> for each field of Update, and the further actions of the TypeSpec. They're the
// values that an API can declare for it.
func (a TypeActions) Allowable() []string {
	names := make([]string, len(a.allowable))
	for i, action := range a.allowable {
		names[i] = action.Name()
	}

	return names
}
