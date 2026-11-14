// Package rules holds the validation rules for free text, such as names, beyond what validation.Required and
// validation.RuneLength check, for the field rules of the services. It depends on no transport, so that the
// services, the HTTP API and the MCP tools can all use it.
package rules

import (
	"strings"
	"unicode"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// Like the length rules, they accept the empty string, which validation.Required rejects.
var (
	// NotBlank rejects text of only whitespace, which looks blank to clients.
	NotBlank = validation.NewStringRuleWithError(
		func(s string) bool { return strings.TrimSpace(s) != "" },
		validation.NewError("validation_not_blank", "cannot be blank"),
	)
	// NoControlCharacters rejects text that contains control characters, such as line breaks or
	// NUL, which PostgreSQL can't store in text.
	NoControlCharacters = validation.NewStringRuleWithError(
		func(s string) bool { return !strings.ContainsFunc(s, unicode.IsControl) },
		validation.NewError("validation_no_control_characters", "must not contain control characters"),
	)
)
