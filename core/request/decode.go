// Package request reads the input of API requests and validates it with ozzo-validation.
// Invalid input is answered with Problem Details.
package request

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"pkgsilo/core/envelope"
)

// MaxBodyBytes is the largest request body DecodeJSON accepts.
const MaxBodyBytes = 1 << 20

// DecodeJSON decodes the JSON request body into dst, which must be a pointer, and validates it.
//
// On failure it writes the problem response and returns false: 415 when the body isn't
// application/json, 413 when it exceeds MaxBodyBytes, 400 when it isn't a single JSON value
// matching dst and 422 with the invalid members when dst fails validation. It panics when dst
// isn't a pointer or a validation rule is misconfigured: bugs in the caller, not in the request.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst validation.Validatable) bool {
	contentType := r.Header.Get("Content-Type")
	if mediaType, _, err := mime.ParseMediaType(contentType); err != nil || mediaType != "application/json" {
		envelope.WriteProblem(w, envelope.UnsupportedMediaTypeProblem.New(envelope.Occurrence{
			Detail: "The request body must be application/json.",
		}))
		return false
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeDecodeProblem(w, err)
		return false
	}
	// A second value, even an empty one, makes the body ambiguous.
	if err := dec.Decode(&json.RawMessage{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errTrailingData
		}
		writeDecodeProblem(w, err)
		return false
	}

	if err := dst.Validate(); err != nil {
		writeValidationProblem(w, err)
		return false
	}

	return true
}

var errTrailingData = errors.New("trailing data after JSON value")

func writeDecodeProblem(w http.ResponseWriter, err error) {
	var (
		maxBytesErr *http.MaxBytesError
		syntaxErr   *json.SyntaxError
		typeErr     *json.UnmarshalTypeError
		invalidErr  *json.InvalidUnmarshalError
	)

	detail := "The request body must be a single JSON object."
	switch {
	case errors.As(err, &maxBytesErr):
		envelope.WriteProblem(w, envelope.RequestEntityTooLargeProblem.New(envelope.Occurrence{
			Detail: fmt.Sprintf("The request body must not exceed %d bytes.", maxBytesErr.Limit),
		}))
		return
	case errors.As(err, &invalidErr):
		// dst isn't a pointer: a bug in the caller, not in the request.
		panic(fmt.Errorf("request: decode body: %w", err))
	case errors.Is(err, io.EOF):
		detail = "The request body must not be empty."
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		detail = "The request body is not valid JSON."
	case errors.As(err, &typeErr) && typeErr.Field != "":
		detail = fmt.Sprintf("The member %q must be of type %s.", typeErr.Field, jsonType(typeErr.Type))
	default:
		// encoding/json reports unknown members only through the message.
		if field, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
			detail = fmt.Sprintf("The member %s is not allowed.", field)
		}
	}

	envelope.WriteProblem(w, envelope.BadRequestProblem.New(envelope.Occurrence{Detail: detail}))
}

// jsonType names the JSON type that decodes into a Go value of type t.
func jsonType(t reflect.Type) string {
	// Such as UUIDs and timestamps, whatever their Go kind.
	if reflect.PointerTo(t).Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) {
		return "string"
	}

	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		return "object"
	}
}

// invalidBodyProblem lists the invalid members of a request body in the
// envelope.ValidationErrorsExtension member, each located by a pointer.
var invalidBodyProblem = envelope.DefineProblemType(
	"invalid-body", "Invalid request body", http.StatusUnprocessableEntity,
)

func writeValidationProblem(w http.ResponseWriter, err error) {
	errs, ok := validationErrors(err, func(path []string) envelope.ValidationError {
		return envelope.ValidationError{Pointer: jsonPointer(path)}
	})
	if !ok {
		panic(fmt.Errorf("request: validate body: %w", err))
	}

	envelope.WriteProblem(w, invalidBodyProblem.New(envelope.Occurrence{
		Detail:     "The request body contains invalid members.",
		Extensions: map[string]any{envelope.ValidationErrorsExtension: errs},
	}))
}

// validationErrors flattens nested ozzo validation errors into one ValidationError per invalid
// value, located by locate from the keys leading to it. It reports false when err isn't a
// validation error but an internal one, such as a misconfigured rule.
func validationErrors(
	err error, locate func(path []string) envelope.ValidationError,
) ([]envelope.ValidationError, bool) {
	var out []envelope.ValidationError

	var flatten func(err error, path []string) bool
	flatten = func(err error, path []string) bool {
		var (
			errs     validation.Errors
			valueErr validation.Error
		)
		switch {
		case errors.As(err, &errs):
			for key, err := range errs {
				if !flatten(err, append(slices.Clone(path), key)) {
					return false
				}
			}
		case errors.As(err, &valueErr):
			ve := locate(path)
			ve.Detail = valueErr.Error()
			out = append(out, ve)
		default:
			return false
		}

		return true
	}
	if !flatten(err, nil) {
		return nil, false
	}

	slices.SortFunc(out, func(a, b envelope.ValidationError) int {
		return strings.Compare(a.Pointer+a.Parameter, b.Pointer+b.Parameter)
	})

	return out, true
}

var pointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

// jsonPointer returns the JSON Pointer to the member at path, in URI fragment form.
func jsonPointer(path []string) string {
	var b strings.Builder
	b.WriteString("#")
	for _, token := range path {
		b.WriteString("/")
		b.WriteString(pointerEscaper.Replace(token))
	}

	return b.String()
}
