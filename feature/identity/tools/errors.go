package tools

import (
	"errors"

	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/identity/service"
)

var (
	// errIdentityNotFound answers a call for an identity that doesn't exist or that the actor may not read,
	// which the service doesn't tell apart.
	errIdentityNotFound = mcpserver.DefineError("identity_not_found",
		"The identity does not exist, or the policies do not allow you to read it.")
	// errUsernameTaken answers a call that gives an identity a username that another identity has.
	errUsernameTaken = mcpserver.DefineError("username_taken",
		"Another identity already has this username. Nothing was changed. Choose another username.")
)

// translateError replaces the service's errors with their tool errors. Denials are left to mcpserver.
func translateError(err error) error {
	switch {
	case errors.Is(err, service.ErrIdentityNotFound):
		return errIdentityNotFound
	case errors.Is(err, service.ErrUsernameTaken):
		return errUsernameTaken
	default:
		return err
	}
}
