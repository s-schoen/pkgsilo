// Package request reads the input of API requests and validates it with ozzo-validation.
// Invalid input is answered with Problem Details.
package request

import (
	"bytes"
	"encoding"
	"encoding/json/jsontext"
	"encoding/json/v2"
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

// MaxBodyBytes is the largest request body DecodeJSON and DecodeMergePatch accept.
const MaxBodyBytes = 1 << 20

// Media types of request bodies.
const (
	jsonMediaType = "application/json"
	// MergePatchMediaType is the media type of a JSON Merge Patch (RFC 7396), which DecodeMergePatch takes.
	MergePatchMediaType = "application/merge-patch+json"
)

// DecodeJSON decodes the JSON request body into dst, which must be a pointer, and validates it.
//
// On failure it writes the problem response and returns false: 415 when the body isn't
// application/json, 413 when it exceeds MaxBodyBytes, 400 when it isn't a single JSON object
// matching dst, null included, and 422 with the invalid members when dst fails validation. It panics when dst
// isn't a pointer or a validation rule is misconfigured: bugs in the caller, not in the request.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst validation.Validatable) bool {
	if !hasMediaType(r, jsonMediaType) {
		envelope.WriteProblem(w, envelope.UnsupportedMediaTypeProblem.New(envelope.Occurrence{
			Detail: "The request body must be application/json.",
		}))
		return false
	}

	return decode(w, r, dst)
}

// DecodeMergePatch decodes the JSON Merge Patch (RFC 7396) in the request body into dst, which must be a
// pointer to a struct, and validates it. Each member of dst is an Optional, so that a member that the patch
// leaves out stays unchanged, and is validated with IfSet, unless null has a meaning of its own.
//
// It answers a failure like DecodeJSON, except that the body must be application/merge-patch+json, which
// the 415 problem names in the Accept-Patch header. The body must be a JSON object too: RFC 7396 lets any
// other value, such as null, replace the whole target, which no endpoint allows.
func DecodeMergePatch(w http.ResponseWriter, r *http.Request, dst validation.Validatable) bool {
	if !hasMediaType(r, MergePatchMediaType) {
		w.Header().Set("Accept-Patch", MergePatchMediaType)
		envelope.WriteProblem(w, envelope.UnsupportedMediaTypeProblem.New(envelope.Occurrence{
			Detail: "The request body must be " + MergePatchMediaType + ".",
		}))
		return false
	}

	return decode(w, r, dst)
}

// hasMediaType reports whether the request body has the media type, whatever its parameters.
func hasMediaType(r *http.Request, want string) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mediaType == want
}

// decode decodes the request body, which must be a single JSON object, into dst and validates it, like
// DecodeJSON.
func decode(w http.ResponseWriter, r *http.Request, dst validation.Validatable) bool {
	if reflect.TypeOf(dst).Kind() != reflect.Pointer {
		// A bug in the caller, not in the request.
		panic(fmt.Sprintf("request: decode body into %T, which is not a pointer", dst))
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	if err == nil {
		err = unmarshalObject(body, dst)
	}
	if err != nil {
		writeDecodeProblem(w, err)
		return false
	}

	if err := dst.Validate(); err != nil {
		writeValidationProblem(w, err)
		return false
	}

	return true
}

// unmarshalObject decodes body, which must be a single JSON object without unknown members, into dst. The
// options apply to every value inside it too, such as the value of an Optional.
func unmarshalObject(body []byte, dst any) error {
	dec := jsontext.NewDecoder(bytes.NewReader(body), json.RejectUnknownMembers(true))
	if err := json.UnmarshalDecode(dec, dst); err != nil {
		return err //nolint:wrapcheck // writeDecodeProblem answers the json package's errors.
	}
	// A second value, even an empty one, makes the body ambiguous.
	if _, err := dec.ReadValue(); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errTrailingData
		}
		return err
	}
	// null decodes into a struct without an error, and changes nothing.
	if !bytes.HasPrefix(bytes.TrimLeft(body, " \t\r\n"), []byte("{")) {
		return errNotObject
	}

	return nil
}

// EnsureEmptyBody is the counterpart of DecodeJSON for endpoints that take no body. On a request
// with a body, it writes a 400 problem and returns false, so that a client doesn't mistake input
// that would be ignored for input that was accepted.
func EnsureEmptyBody(w http.ResponseWriter, r *http.Request) bool {
	// A single byte tells an empty body apart from any other.
	if _, err := io.ReadFull(r.Body, make([]byte, 1)); errors.Is(err, io.EOF) {
		return true
	}

	envelope.WriteProblem(w, envelope.BadRequestProblem.New(envelope.Occurrence{
		Detail: "The request body must be empty.",
	}))

	return false
}

var (
	errTrailingData = errors.New("trailing data after JSON value")
	errNotObject    = errors.New("JSON value is not an object")
)

// writeDecodeProblem answers err, which reading or decoding the request body failed with.
func writeDecodeProblem(w http.ResponseWriter, err error) {
	if maxBytesErr, ok := errors.AsType[*http.MaxBytesError](err); ok {
		envelope.WriteProblem(w, envelope.RequestEntityTooLargeProblem.New(envelope.Occurrence{
			Detail: fmt.Sprintf("The request body must not exceed %d bytes.", maxBytesErr.Limit),
		}))
		return
	}

	envelope.WriteProblem(w, envelope.BadRequestProblem.New(envelope.Occurrence{Detail: decodeProblemDetail(err)}))
}

// decodeProblemDetail explains to the client why decoding the request body failed with err.
func decodeProblemDetail(err error) string {
	syntaxErr, isSyntax := errors.AsType[*jsontext.SyntacticError](err)
	semanticErr, isSemantic := errors.AsType[*json.SemanticError](err)
	switch {
	case errors.Is(err, io.EOF):
		return "The request body must not be empty."
	case isSyntax && errors.Is(err, jsontext.ErrDuplicateName):
		return fmt.Sprintf("The member %q appears more than once.", memberName(syntaxErr.JSONPointer))
	case isSyntax, errors.Is(err, io.ErrUnexpectedEOF):
		return "The request body is not valid JSON."
	case isSemantic && errors.Is(err, json.ErrUnknownName):
		return fmt.Sprintf("The member %q is not allowed.", memberName(semanticErr.JSONPointer))
	case isSemantic && isTypeError(semanticErr):
		return fmt.Sprintf("The member %q must be of type %s.",
			memberName(semanticErr.JSONPointer), jsonType(semanticErr.GoType))
	default:
		// Such as a value that isn't an object, or a value of the right type that its Go type rejects, such
		// as a malformed UUID.
		return "The request body must be a single JSON object."
	}
}

// isTypeError reports whether err is about a member whose JSON value is of another type than its Go type
// takes. A value that isn't a member, such as a body that's an array, is no type error.
func isTypeError(err *json.SemanticError) bool {
	return err.JSONPointer != "" && err.GoType != nil && kindName(err.JSONKind) != jsonType(err.GoType)
}

// memberName names the member that ptr points to by its path, with its names joined by dots, such as
// "address.city".
func memberName(ptr jsontext.Pointer) string {
	return strings.Join(slices.Collect(ptr.Tokens()), ".")
}

// The names of the JSON types, which problem details name a member's type by.
const (
	jsonString  = "string"
	jsonNumber  = "number"
	jsonBoolean = "boolean"
	jsonArray   = "array"
	jsonObject  = "object"
	jsonNull    = "null"
)

// kindName names the JSON type of a value of the kind, as jsonType does.
func kindName(kind jsontext.Kind) string {
	switch kind {
	case '"':
		return jsonString
	case '0':
		return jsonNumber
	case 't', 'f':
		return jsonBoolean
	case '[':
		return jsonArray
	case 'n':
		return jsonNull
	default:
		return jsonObject
	}
}

// jsonType names the JSON type that decodes into a Go value of type t.
func jsonType(t reflect.Type) string {
	// Such as UUIDs and timestamps, whatever their Go kind.
	if reflect.PointerTo(t).Implements(reflect.TypeFor[encoding.TextUnmarshaler]()) {
		return jsonString
	}

	switch t.Kind() {
	case reflect.String:
		return jsonString
	case reflect.Bool:
		return jsonBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return jsonNumber
	case reflect.Slice, reflect.Array:
		return jsonArray
	default:
		return jsonObject
	}
}

var invalidBodyType = envelope.DefineProblemType("invalid-body", "Invalid request body", http.StatusUnprocessableEntity)

// InvalidBodyProblem returns the problem that DecodeJSON answers a body with invalid members with.
// It lists errs, each located by a pointer, in the envelope.ValidationErrorsExtension member.
func InvalidBodyProblem(errs ...envelope.ValidationError) envelope.Problem {
	return invalidBodyType.New(envelope.Occurrence{
		Detail:     "The request body contains invalid members.",
		Extensions: map[string]any{envelope.ValidationErrorsExtension: errs},
	})
}

func writeValidationProblem(w http.ResponseWriter, err error) {
	errs, ok := validationErrors(err, func(path []string) envelope.ValidationError {
		return envelope.ValidationError{Pointer: jsonPointer(path)}
	})
	if !ok {
		panic(fmt.Errorf("request: validate body: %w", err))
	}

	envelope.WriteProblem(w, InvalidBodyProblem(errs...))
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
		if errs, ok := errors.AsType[validation.Errors](err); ok {
			for key, err := range errs {
				if !flatten(err, append(slices.Clone(path), key)) {
					return false
				}
			}

			return true
		}
		if valueErr, ok := errors.AsType[validation.Error](err); ok {
			ve := locate(path)
			ve.Detail = valueErr.Error()
			out = append(out, ve)

			return true
		}

		return false
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
