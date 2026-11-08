package service

import "pkgsilo/core/authz"

// The actions on groups. Each settable field has its own action for each kind of write, such as
// group:update:name, so that policies can limit writes to some fields. The members of a group are managed
// with actions on the group, and reading them is one action for the whole member list. Each field but the ID
// has its own action for reading it too, such as group:read:name, on top of group:read.
var (
	actionRead         = authz.NewAction("group:read", authz.ActionGroupRead)
	actionDelete       = authz.NewAction("group:delete")
	actionReadMembers  = authz.NewAction("group:readMembers", authz.ActionGroupRead)
	actionAddMember    = authz.NewAction("group:addMember")
	actionRemoveMember = authz.NewAction("group:removeMember")
	readFields         = authz.NewReadFieldActions("group", "name", "createdBy", "updatedBy", "createdAt", "updatedAt")
	createFields       = authz.NewFieldActions("group:create", "name")
	updateFields       = authz.NewFieldActions("group:update", "name")
)

// Declaration declares groups and the actions on them, for the schema that policies are validated against.
func Declaration() authz.Declaration {
	return authz.Declaration{
		Type:    authz.GroupType,
		Actions: []authz.Action{actionRead, actionDelete, actionReadMembers, actionAddMember, actionRemoveMember},
		Fields:  []authz.FieldActions{readFields, createFields, updateFields},
	}
}

// ReadableFields returns the names of the fields of a group, besides its ID, each of which has a
// group:read:<field> action.
func ReadableFields() []string {
	return readFields.Fields()
}

// CreatableFields returns the names of the fields that creating a group sets, each of which has a
// group:create:<field> action.
func CreatableFields() []string {
	return createFields.Fields()
}

// UpdatableFields returns the names of the fields that replacing a group sets, and patching it may set, each
// of which has a group:update:<field> action.
func UpdatableFields() []string {
	return updateFields.Fields()
}
