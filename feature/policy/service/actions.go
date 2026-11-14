package service

import (
	"reflect"

	"pkgsilo/core/authz"
)

// actionReadSchema is the action of reading the Cedar schema that policies are validated against, which
// names every entity type and action. It's a reading action, so that a policy that permits read permits it
// too.
var actionReadSchema = authz.NewAction("policy:readSchema", authz.ActionGroupRead)

// The actions on policies, of the Cedar entity type Policy: policy:read, policy:delete,
// policy:readSchema, and an action for each field and kind of access, named after the authz tags of Policy
// and PolicyInput, such as policy:read:source and policy:update:source, so that policies can limit access
// to some fields.
var actions = authz.DeclareType(authz.TypeSpec{
	Type:    "Policy",
	Model:   reflect.TypeFor[Policy](),
	Create:  reflect.TypeFor[PolicyInput](),
	Update:  reflect.TypeFor[PolicyInput](),
	Patch:   reflect.TypeFor[PolicyPatch](),
	Actions: []authz.Action{actionReadSchema},
})

// Actions returns the actions on policies, for the schema that policies are validated against and for the
// handler tests.
func Actions() authz.TypeActions {
	return actions
}
