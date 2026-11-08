package service

import (
	"reflect"

	"pkgsilo/core/authz"
)

// The actions on groups: group:read, group:delete, and an action for each field and kind of access, named
// after the authz tags of Group and GroupInput, such as group:read:name and group:update:name, so that
// policies can limit access to some fields. The members of a group are managed with actions on the group,
// and reading them is one action for the whole member list.
var (
	actionReadMembers  = authz.NewAction("group:readMembers", authz.ActionGroupRead)
	actionAddMember    = authz.NewAction("group:addMember")
	actionRemoveMember = authz.NewAction("group:removeMember")

	actions = authz.DeclareType(authz.TypeSpec{
		Type:    authz.GroupType,
		Model:   reflect.TypeFor[Group](),
		Create:  reflect.TypeFor[GroupInput](),
		Update:  reflect.TypeFor[GroupInput](),
		Patch:   reflect.TypeFor[GroupPatch](),
		Actions: []authz.Action{actionReadMembers, actionAddMember, actionRemoveMember},
	})
)

// Actions returns the actions on groups, for the schema that policies are validated against and for the
// handler tests.
func Actions() authz.TypeActions {
	return actions
}
