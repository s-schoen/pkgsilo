package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"

	"pkgsilo/core/authz"
	"pkgsilo/core/middleware"
)

// errorCodePattern is the form of a tool error's code, in snake case, such as identity_not_found.
var errorCodePattern = regexp.MustCompile(`^[a-z]+(_[a-z]+)*$`)

// ToolError is an error of a tool that a client tells apart by its code, the counterpart of a problem
// type of the HTTP API. It's answered as a tool result that is an error, whose text is
// "<code>: <message>", so that the model reads it and can react.
type ToolError struct {
	// Code identifies the error, in snake case, such as identity_not_found.
	Code string
	// Message tells the model what went wrong, and what it can do about it.
	Message string
}

// DefineError returns the tool error with the given code and message, for a feature's tools/errors.go,
// whose translateError answers the service's errors with them. It panics if the code isn't in snake case
// or the message is empty.
func DefineError(code, message string) *ToolError {
	if !errorCodePattern.MatchString(code) {
		panic(fmt.Sprintf("mcpserver: error code %q is not in snake case", code))
	}
	if message == "" {
		panic(fmt.Sprintf("mcpserver: error %q has no message", code))
	}

	return &ToolError{Code: code, Message: message}
}

// Error returns the text of the tool result that answers the error.
func (e *ToolError) Error() string {
	return e.Code + ": " + e.Message
}

// The codes of the tool errors that Registry.Add answers itself, whatever the feature.
const (
	invalidArgumentsCode = "invalid_arguments"
	fieldsDeniedCode     = "fields_denied"
	forbiddenCode        = "forbidden"
	internalErrorCode    = "internal_error"
)

// WriteErrorsDescription tells an agent what fields_denied and invalid_arguments mean, which Registry.Add
// answers itself. It ends the description of a tool that creates or changes an object.
const WriteErrorsDescription = "Fails with fields_denied, naming the arguments, if the policies do not allow " +
	"you to set some of them, and with invalid_arguments, naming each invalid argument and why, if they break " +
	"the rules; nothing is changed then."

// ErrForbidden answers authz.ErrDenied: the policies don't allow the action, though the actor may read the
// object.
var ErrForbidden = DefineError(forbiddenCode, "The policies do not allow this action.")

// FieldsDeniedError returns the error that answers an *authz.FieldsDeniedError, naming the fields that the
// arguments set and the actor may not. Nothing was written.
func FieldsDeniedError(fields ...string) *ToolError {
	return DefineError(fieldsDeniedCode,
		"The policies do not allow setting these arguments: "+strings.Join(fields, ", ")+". Nothing was changed.")
}

// InvalidArgumentsError returns the error that answers arguments that don't match the input schema or fail
// validation, naming each invalid member and why, as errs does.
func InvalidArgumentsError(errs validation.Errors) *ToolError {
	return DefineError(invalidArgumentsCode, "The arguments are invalid: "+errs.Error())
}

// InternalError returns the error that answers an unexpected error of a call with the given request ID,
// which users can quote. It says nothing about the cause, which is logged.
func InternalError(requestID string) *ToolError {
	return DefineError(internalErrorCode, "internal error (request ID "+requestID+")")
}

// toolError returns the tool error that answers the error that the handler of the tool with the given name
// returned (see Registry.Add). It logs an unexpected error with the call's context.
func (r *Registry) toolError(ctx context.Context, tool string, err error) *ToolError {
	if toolErr, ok := errors.AsType[*ToolError](err); ok {
		return toolErr
	}
	if fieldsDenied, ok := errors.AsType[*authz.FieldsDeniedError](err); ok {
		return FieldsDeniedError(fieldsDenied.Fields...)
	}
	if errors.Is(err, authz.ErrDenied) {
		return ErrForbidden
	}

	r.logger.ErrorContext(ctx, "mcp tool call failed", "tool", tool, "error", err)

	return InternalError(middleware.RequestIDFromContext(ctx))
}

// internalJSONRPCError returns the JSON-RPC error that answers an unexpected error of an MCP method with
// the given request ID, as InternalError does for a tool.
func internalJSONRPCError(requestID string) *jsonrpc.Error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: InternalError(requestID).Message}
}
