package tools

import (
	"errors"

	identitytools "pkgsilo/feature/identity/tools"
	"pkgsilo/feature/permission/service"
)

// translateError replaces the service's errors with their tool errors. Denials are left to mcpserver.
func translateError(err error) error {
	if errors.Is(err, service.ErrIdentityNotFound) {
		// The error of the identity tools, so that agents recognize a missing identity in one way.
		return identitytools.ErrIdentityNotFound
	}

	return err
}
