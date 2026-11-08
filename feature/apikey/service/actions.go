package service

import "pkgsilo/core/authz"

// The actions on API keys. API keys can't be changed, so creating is the only write with field actions. Each
// field but the ID has its own action for reading it, such as apikey:read:name, on top of apikey:read. The
// key itself is no field: it's only returned when it's created, to its creator. An API key belongs to its
// identity, which policies refer to as resource.owner.
var (
	actionRead   = authz.NewAction("apikey:read", authz.ActionGroupRead)
	actionDelete = authz.NewAction("apikey:delete")
	readFields   = authz.NewReadFieldActions("apikey",
		"identityId", "name", "keySuffix", "expiresAt", "lastUsedAt", "createdBy", "createdAt")
	createFields = authz.NewFieldActions("apikey:create", "name", "expiresAt")
)

// resourceType is the Cedar entity type of API keys. Its key hash isn't an attribute, nor is anything
// else of it that's secret.
const resourceType = "APIKey"

// Declaration declares API keys, owned by their identities, and the actions on them, for the schema that
// policies are validated against.
func Declaration() authz.Declaration {
	return authz.Declaration{
		Type:    resourceType,
		Owned:   true,
		Actions: []authz.Action{actionRead, actionDelete},
		Fields:  []authz.FieldActions{readFields, createFields},
	}
}

// ReadableFields returns the names of the fields of an API key, besides its ID, each of which has an
// apikey:read:<field> action.
func ReadableFields() []string {
	return readFields.Fields()
}

// CreatableFields returns the names of the fields that creating an API key can set, each of which has an
// apikey:create:<field> action.
func CreatableFields() []string {
	return createFields.Fields()
}
