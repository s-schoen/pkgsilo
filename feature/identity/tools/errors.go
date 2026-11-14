package tools

import (
	"errors"

	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/identity/service"
)

// errIdentityNotFound answers a call for an identity that doesn't exist or that the actor may not read,
// which the service doesn't tell apart.
var errIdentityNotFound = mcpserver.DefineError("identity_not_found",
	"The identity does not exist, or the policies do not allow you to read it.")

// translateError replaces the service's errors with their tool errors. Denials are left to mcpserver.
func translateError(err error) error {
	switch {
	case errors.Is(err, service.ErrIdentityNotFound):
		return errIdentityNotFound
	default:
		return err
	}
}
