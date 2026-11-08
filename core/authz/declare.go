package authz

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
)

// fieldTag is the struct tag that names a field of an object, which its field actions are named after,
// such as `authz:"username"` for identity:read:username.
const fieldTag = "authz"

// fieldName is the format of a field name: an action name holds it after a colon, so it can't hold one.
var fieldName = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)

// TypeSpec describes a resource type by the Go types of its feature, from whose authz tags DeclareType
// derives its field actions. The actions are named after the type in lower case, such as apikey:read for
// the type APIKey.
type TypeSpec struct {
	// Type is the Cedar entity type of the resources, such as Identity.
	Type string
	// Owned declares that each resource belongs to an identity, as Declaration.Owned does.
	Owned bool
	// Model is the domain model as an actor sees it: every member but ID is a Field, tagged with the name of
	// the field, such as `authz:"username"`. Each one gets an action <type>:read:<field>.
	Model reflect.Type
	// Create is the input that creating a resource sets, if creating it sets fields: every member is tagged
	// with the name of the field, and gets an action <type>:create:<field>.
	Create reflect.Type
	// Update is the input that replacing a resource sets, if it can be changed: every member is tagged with
	// the name of the field, and gets an action <type>:update:<field>.
	Update reflect.Type
	// Patch is the input that patching a resource sets, if it can be patched: every member is a pointer,
	// tagged like the member of Update that it changes, in the same order.
	Patch reflect.Type
	// Actions are the further actions on the resources, such as group:addMember.
	Actions []Action
}

// TypeActions are the actions on a resource type that DeclareType derived from its TypeSpec.
type TypeActions struct {
	declaration  Declaration
	model        reflect.Type
	read, delete Action
	readFields   FieldActions
	createFields FieldActions
	updateFields FieldActions
}

// DeclareType declares the resource type of spec and the actions on it: <type>:read, as a member of the
// action group read, <type>:delete, the field actions of its Model, Create and Update, and its further
// Actions. It panics on a spec whose types break the rules of TypeSpec, which is a bug that a feature's
// package initialization reveals: a member without an action would be revealed or written unchecked.
func DeclareType(spec TypeSpec) TypeActions {
	prefix := strings.ToLower(spec.Type)
	actions := TypeActions{
		model:      spec.Model,
		read:       NewAction(prefix+":read", ActionGroupRead),
		delete:     NewAction(prefix + ":delete"),
		readFields: NewReadFieldActions(prefix, modelFields(spec.Model)...),
	}
	if spec.Create != nil {
		actions.createFields = NewFieldActions(prefix+":create", inputFields(spec.Create)...)
	}
	if spec.Update != nil {
		actions.updateFields = NewFieldActions(prefix+":update", inputFields(spec.Update)...)
	}
	if spec.Patch != nil {
		checkPatch(spec.Patch, spec.Update)
	}

	actions.declaration = Declaration{
		Type:    spec.Type,
		Owned:   spec.Owned,
		Actions: append([]Action{actions.read, actions.delete}, spec.Actions...),
		Fields:  []FieldActions{actions.readFields, actions.createFields, actions.updateFields},
	}

	return actions
}

// Type returns the Cedar entity type of the resources.
func (a TypeActions) Type() string {
	return a.declaration.Type
}

// Read returns the action <type>:read of reading a resource, which reveals only its ID.
func (a TypeActions) Read() Action {
	return a.read
}

// Delete returns the action <type>:delete of deleting a resource.
func (a TypeActions) Delete() Action {
	return a.delete
}

// ReadFields returns the actions <type>:read:<field> of reading each field of the Model.
func (a TypeActions) ReadFields() FieldActions {
	return a.readFields
}

// CreateFields returns the actions <type>:create:<field> of setting each field of Create, none without it.
func (a TypeActions) CreateFields() FieldActions {
	return a.createFields
}

// UpdateFields returns the actions <type>:update:<field> of setting each field of Update, none without it.
func (a TypeActions) UpdateFields() FieldActions {
	return a.updateFields
}

// Declaration returns the declaration of the resource type and all its actions, for Schema.
func (a TypeActions) Declaration() Declaration {
	return a.declaration
}

// SetFields returns the names of the fields that the input v sets, in the order of its members: those of
// its pointer members that aren't nil, and all others. v is a struct whose members are tagged with the
// names of their fields, such as a TypeSpec's Create or Patch, so that a write that sets only some fields
// checks the actions of those, with FieldActions.Only. It panics on a member without a tag, which is a bug
// in the type: the field would be written unchecked.
func SetFields(v any) []string {
	value := reflect.ValueOf(v)
	names := inputFields(value.Type())

	var set []string
	for i, name := range names {
		member := value.Field(i)
		if member.Kind() != reflect.Pointer || !member.IsNil() {
			set = append(set, name)
		}
	}

	return set
}

// modelFields returns the names of the fields of the domain model t, every member but ID, each of which
// must be a Field.
func modelFields(t reflect.Type) []string {
	checkStruct(t)

	var names []string
	for member := range t.Fields() {
		if member.Name == "ID" {
			continue
		}
		if !isField(member.Type) {
			panic(fmt.Sprintf("authz: member %s of %s is a %s, not an authz.Field, so it can't be hidden",
				member.Name, t, member.Type))
		}
		names = append(names, tagName(t, member))
	}

	return checkUnique(t, names)
}

// isField reports whether t is a Field.
func isField(t reflect.Type) bool {
	return t.PkgPath() == reflect.TypeFor[Field[any]]().PkgPath() && strings.HasPrefix(t.Name(), "Field[")
}

// inputFields returns the names of the fields of the input t, one for each member.
func inputFields(t reflect.Type) []string {
	checkStruct(t)

	var names []string
	for member := range t.Fields() {
		names = append(names, tagName(t, member))
	}

	return checkUnique(t, names)
}

// checkPatch checks that every member of the patch is a pointer, and that it has the fields of update.
func checkPatch(patch, update reflect.Type) {
	if update == nil {
		panic(fmt.Sprintf("authz: patch %s of a type without an update", patch))
	}

	got, want := inputFields(patch), inputFields(update)
	for member := range patch.Fields() {
		if member.Type.Kind() != reflect.Pointer {
			panic(fmt.Sprintf("authz: member %s of %s is a %s, not a pointer, so it can't be left out",
				member.Name, patch, member.Type))
		}
	}
	if !slices.Equal(got, want) {
		panic(fmt.Sprintf("authz: the fields %v of %s aren't the fields %v of %s", got, patch, want, update))
	}
}

// tagName returns the name of the field that the member of t is tagged with.
func tagName(t reflect.Type, member reflect.StructField) string {
	name := member.Tag.Get(fieldTag)
	if !fieldName.MatchString(name) {
		panic(fmt.Sprintf("authz: member %s of %s has the field name %q, not a tag `authz:\"<field>\"` with a "+
			"name in lower camel case", member.Name, t, name))
	}

	return name
}

// checkUnique returns the names of the fields of t, after checking that no two members share one.
func checkUnique(t reflect.Type, names []string) []string {
	for i, name := range names {
		if slices.Contains(names[:i], name) {
			panic(fmt.Sprintf("authz: more than one member of %s has the field name %q", t, name))
		}
	}

	return names
}

// checkStruct checks that t is a struct type.
func checkStruct(t reflect.Type) {
	if t == nil || t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("authz: %v is no struct type", t))
	}
}
