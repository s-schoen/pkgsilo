package service

import (
	"reflect"

	"pkgsilo/core/authz"
)

// The actions on policies, of the Cedar entity type Policy: policy:read, policy:delete, and an action for
// each field and kind of access, named after the authz tags of Policy and PolicyInput, such as
// policy:read:source and policy:update:source, so that policies can limit access to some fields.
var actions = authz.DeclareType(authz.TypeSpec{
	Type:   "Policy",
	Model:  reflect.TypeFor[Policy](),
	Create: reflect.TypeFor[PolicyInput](),
	Update: reflect.TypeFor[PolicyInput](),
	Patch:  reflect.TypeFor[PolicyPatch](),
})

// Actions returns the actions on policies, for the schema that policies are validated against and for the
// handler tests.
func Actions() authz.TypeActions {
	return actions
}
