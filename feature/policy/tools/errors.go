package tools

import (
	"errors"

	"pkgsilo/core/mcpserver"
	"pkgsilo/feature/policy/service"
)

var (
	// errPolicyNotFound answers a call for a policy that doesn't exist or that the actor may not read, which
	// the service doesn't tell apart.
	errPolicyNotFound = mcpserver.DefineError("policy_not_found",
		"The policy does not exist, or the policies do not allow you to read it.")
	// errPolicyNameTaken answers a call that gives a policy a name that another policy has.
	errPolicyNameTaken = mcpserver.DefineError("policy_name_taken",
		"Another policy already has this name. Nothing was changed. Choose another name.")
)

// invalidPolicyError returns the error that answers a policy that can't be saved, with the reason of the
// service's checks, which tells the model what to fix.
func invalidPolicyError(reason string) *mcpserver.ToolError {
	return mcpserver.DefineError("invalid_policy", "The policy is invalid: "+reason+". Nothing was changed. "+
		"Fix the source, with the schema that policy_get_schema returns, and call again.")
}

// translateError replaces the service's errors with their tool errors. Denials are left to mcpserver.
func translateError(err error) error {
	if invalid, ok := errors.AsType[*service.InvalidPolicyError](err); ok {
		return invalidPolicyError(invalid.Reason)
	}

	switch {
	case errors.Is(err, service.ErrPolicyNotFound):
		return errPolicyNotFound
	case errors.Is(err, service.ErrPolicyNameTaken):
		return errPolicyNameTaken
	default:
		return err
	}
}
