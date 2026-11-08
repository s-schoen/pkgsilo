package service

import (
	"reflect"

	"pkgsilo/core/authz"
)

// The actions on API keys, of the Cedar entity type APIKey: apikey:read, apikey:delete, and an action for
// each field and kind of access, named after the authz tags of APIKey and APIKeyInput, such as
// apikey:read:name and apikey:create:name. API keys can't be changed, so creating is the only write with
// field actions. The key itself is no field: it's only returned when it's created, to its creator, and
// neither it nor its hash is an attribute. An API key belongs to its identity, which policies refer to as
// resource.owner.
var actions = authz.DeclareType(authz.TypeSpec{
	Type:   "APIKey",
	Owned:  true,
	Model:  reflect.TypeFor[APIKey](),
	Create: reflect.TypeFor[APIKeyInput](),
})

// Actions returns the actions on API keys, for the schema that policies are validated against and for the
// handler tests.
func Actions() authz.TypeActions {
	return actions
}
