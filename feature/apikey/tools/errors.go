package tools

import (
	"errors"

	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/apikey/service"
	identitytools "pkgsilo/feature/identity/tools"
)

// errAPIKeyNotFound answers a call for an API key that the identity doesn't have, or that the actor may not
// read, which the service doesn't tell apart.
var errAPIKeyNotFound = mcpserver.DefineError("apikey_not_found",
	"The identity has no API key with this ID, or the policies do not allow you to read it.")

// translateError replaces the service's errors with their tool errors. Denials are left to mcpserver.
func translateError(err error) error {
	switch {
	case errors.Is(err, service.ErrAPIKeyNotFound):
		return errAPIKeyNotFound
	case errors.Is(err, service.ErrIdentityNotFound):
		// The error of the identity tools, so that agents recognize a missing identity in one way.
		return identitytools.ErrIdentityNotFound
	default:
		return err
	}
}
