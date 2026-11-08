package service

import "pkgsilo/core/authz"

// policyType is the Cedar entity type of policies.
const policyType = "Policy"

// The actions on policies. Each settable field has its own action for each kind of write, such as
// policy:update:source, so that policies can limit writes to some fields. Each field but the ID has its own
// action for reading it too, such as policy:read:source, on top of policy:read.
var (
	actionRead   = authz.NewAction("policy:read", authz.ActionGroupRead)
	actionDelete = authz.NewAction("policy:delete")
	readFields   = authz.NewReadFieldActions("policy",
		"name", "source", "createdBy", "updatedBy", "createdAt", "updatedAt")
	createFields = authz.NewFieldActions("policy:create", "name", "source")
	updateFields = authz.NewFieldActions("policy:update", "name", "source")
)

// Declaration declares policies and the actions on them, for the schema that policies are validated
// against.
func Declaration() authz.Declaration {
	return authz.Declaration{
		Type:    policyType,
		Actions: []authz.Action{actionRead, actionDelete},
		Fields:  []authz.FieldActions{readFields, createFields, updateFields},
	}
}

// ReadableFields returns the names of the fields of a policy, besides its ID, each of which has a
// policy:read:<field> action.
func ReadableFields() []string {
	return readFields.Fields()
}

// CreatableFields returns the names of the fields that creating a policy sets, each of which has a
// policy:create:<field> action.
func CreatableFields() []string {
	return createFields.Fields()
}

// UpdatableFields returns the names of the fields that replacing a policy sets, and patching it may set,
// each of which has a policy:update:<field> action.
func UpdatableFields() []string {
	return updateFields.Fields()
}
