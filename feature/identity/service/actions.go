package service

import (
	"reflect"

	"pkgsilo/core/authz"
)

// The actions on identities: identity:read, identity:delete, and an action for each field and kind of
// access, named after the authz tags of Identity and IdentityInput, such as identity:read:username and
// identity:update:username, so that policies can limit access to some fields. Creating has actions of its
// own, so that an actor who may rename itself can't create identities.
var actions = authz.DeclareType(authz.TypeSpec{
	Type:   authz.IdentityType,
	Model:  reflect.TypeFor[Identity](),
	Create: reflect.TypeFor[IdentityInput](),
	Update: reflect.TypeFor[IdentityInput](),
	Patch:  reflect.TypeFor[IdentityPatch](),
})

// Actions returns the actions on identities, for the schema that policies are validated against, for the
// handler tests, and for other features: they check its Read action on an identity that a request refers
// to, such as a new group member, and report an identity that the actor may not read like a missing one.
// The identity as the resource must carry its groups.
func Actions() authz.TypeActions {
	return actions
}
