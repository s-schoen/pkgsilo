package service

import (
	"reflect"

	"pkgsilo/core/authz"
)

// The actions on passwords, of the Cedar entity type Password: password:read, password:delete,
// password:read:setAt, and two ways of setting a password. A password belongs to its identity, which
// policies refer to as resource.owner, and its ID is the identity's. Neither the password nor its hash is an
// attribute.
var (
	// changeAction sets a password that exists, with the current one.
	changeAction = authz.NewAction("password:change")
	// resetAction sets a password without the current one, and is the only way to set a first password.
	resetAction = authz.NewAction("password:reset")

	actions = authz.DeclareType(authz.TypeSpec{
		Type:    "Password",
		Owned:   true,
		Model:   reflect.TypeFor[Password](),
		Actions: []authz.Action{changeAction, resetAction},
	})
)

// Actions returns the actions on passwords, for the schema that policies are validated against and for the
// handler tests.
func Actions() authz.TypeActions {
	return actions
}
