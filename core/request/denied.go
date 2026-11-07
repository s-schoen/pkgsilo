package request

import (
	"net/http"

	"pkgsilo/core/envelope"
)

var fieldsDeniedType = envelope.DefineProblemType("fields-denied", "Fields denied", http.StatusForbidden)

// FieldsDeniedProblem returns the problem that answers a write of fields that the caller may not set,
// such as those of an authz.FieldsDeniedError. It lists each field, located by a pointer to the request
// body's member of that name, in the envelope.ValidationErrorsExtension member. Nothing was written.
func FieldsDeniedProblem(fields ...string) envelope.Problem {
	errs := make([]envelope.ValidationError, len(fields))
	for i, field := range fields {
		errs[i] = envelope.ValidationError{Pointer: jsonPointer([]string{field}), Detail: "may not be set by the caller"}
	}

	return fieldsDeniedType.New(envelope.Occurrence{
		Detail:     "The request body sets members that the caller may not set.",
		Extensions: map[string]any{envelope.ValidationErrorsExtension: errs},
	})
}
