package service

import "pkgsilo/core/authz"

// The actions on identities. Each settable field has its own action for each kind of write, such as
// identity:update:username, so that policies can limit writes to some fields. Creating has actions of its
// own, so that an actor who may rename itself can't create identities. Each field but the ID has its own
// action for reading it too, such as identity:read:username, on top of identity:read.
var (
	actionRead   = authz.NewAction("identity:read", authz.ActionGroupRead)
	actionDelete = authz.NewAction("identity:delete")
	readFields   = authz.NewReadFieldActions("identity",
		"username", "displayName", "createdBy", "updatedBy", "createdAt", "updatedAt")
	createFields = authz.NewFieldActions("identity:create", "username", "displayName")
	updateFields = authz.NewFieldActions("identity:update", "username", "displayName")
)

// Declaration declares identities and the actions on them, for the schema that policies are validated
// against.
func Declaration() authz.Declaration {
	return authz.Declaration{
		Type:    authz.IdentityType,
		Actions: []authz.Action{actionRead, actionDelete},
		Fields:  []authz.FieldActions{readFields, createFields, updateFields},
	}
}

// ReadAction returns the action of reading an identity. Other features check it on an identity that a
// request refers to, such as a new group member, and report an identity that the actor may not read like
// a missing one. The identity as the resource must carry its groups.
func ReadAction() authz.Action {
	return actionRead
}

// ReadableFields returns the names of the fields of an identity, besides its ID, each of which has an
// identity:read:<field> action.
func ReadableFields() []string {
	return readFields.Fields()
}

// CreatableFields returns the names of the fields that creating an identity sets, each of which has an
// identity:create:<field> action.
func CreatableFields() []string {
	return createFields.Fields()
}

// UpdatableFields returns the names of the fields that replacing an identity sets, and patching it may set,
// each of which has an identity:update:<field> action.
func UpdatableFields() []string {
	return updateFields.Fields()
}
