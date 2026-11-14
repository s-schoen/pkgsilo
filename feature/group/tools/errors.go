package tools

import (
	"errors"

	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/group/service"
	identitytools "pkgsilo/feature/identity/tools"
)

var (
	// errGroupNotFound answers a call for a group that doesn't exist or that the actor may not read, which
	// the service doesn't tell apart.
	errGroupNotFound = mcpserver.DefineError("group_not_found",
		"The group does not exist, or the policies do not allow you to read it.")
	// errGroupNameTaken answers a call that gives a group a name that another group has.
	errGroupNameTaken = mcpserver.DefineError("group_name_taken",
		"Another group already has this name. Nothing was changed. Choose another name.")
	// errMemberNotFound answers a call that removes an identity that isn't a member of the group, or that
	// the actor may not read, which the service doesn't tell apart.
	errMemberNotFound = mcpserver.DefineError("member_not_found",
		"The identity is not a member of the group, or the policies do not allow you to read it. Nothing was "+
			"changed.")
)

// translateError replaces the service's errors with their tool errors. Denials are left to mcpserver.
func translateError(err error) error {
	switch {
	case errors.Is(err, service.ErrGroupNotFound):
		return errGroupNotFound
	case errors.Is(err, service.ErrGroupNameTaken):
		return errGroupNameTaken
	case errors.Is(err, service.ErrIdentityNotFound):
		// The error of the identity tools, so that agents recognize a missing identity in one way.
		return identitytools.ErrIdentityNotFound
	case errors.Is(err, service.ErrMemberNotFound):
		return errMemberNotFound
	default:
		return err
	}
}
