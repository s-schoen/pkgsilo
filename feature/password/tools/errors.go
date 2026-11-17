package tools

import (
	"errors"

	"pkgsilo/core/mcpserver"
	identitytools "pkgsilo/feature/identity/tools"
	"pkgsilo/feature/password/service"
)

// errPasswordNotFound answers a call for the password of an identity that has none, or whose password the
// actor may not read, which the service doesn't tell apart.
var errPasswordNotFound = mcpserver.DefineError("password_not_found",
	"The identity has no password, or the policies do not allow you to read it.")

// translateError replaces the service's errors with their tool errors. Denials are left to mcpserver.
func translateError(err error) error {
	switch {
	case errors.Is(err, service.ErrPasswordNotFound):
		return errPasswordNotFound
	case errors.Is(err, service.ErrIdentityNotFound):
		// The error of the identity tools, so that agents recognize a missing identity in one way.
		return identitytools.ErrIdentityNotFound
	default:
		return err
	}
}
