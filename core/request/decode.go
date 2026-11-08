// Package request reads the input of API requests and validates it with ozzo-validation.
// Invalid input is answered with Problem Details.
package request

import (
	"bytes"
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
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	if err != nil {
		writeDecodeProblem(w, err, body, dst)
		return false
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeDecodeProblem(w, err, body, dst)
		return false
	}
	// A second value, even an empty one, makes the body ambiguous.
	if err := dec.Decode(&json.RawMessage{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errTrailingData
		}
		writeDecodeProblem(w, err, body, dst)
		return false
	}
	// null decodes into a struct without an error, and changes nothing.
	if !bytes.HasPrefix(bytes.TrimLeft(body, " \t\r\n"), []byte("{")) {
		writeDecodeProblem(w, errNotObject, body, dst)
		return false
	}

	if err := dst.Validate(); err != nil {
		writeValidationProblem(w, err)
		return false
	}

	return true
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

// writeDecodeProblem answers err, which decoding body into dst failed with.
func writeDecodeProblem(w http.ResponseWriter, err error, body []byte, dst any) {
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
	case errors.As(err, &typeErr):
		if member, ok := typeErrorMember(typeErr, body, dst); ok {
			detail = fmt.Sprintf("The member %q must be of type %s.", member, jsonType(typeErr.Type))
		}
	default:
		// encoding/json reports unknown members only through the message.
		if field, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
			detail = fmt.Sprintf("The member %s is not allowed.", field)
		}
	}

	envelope.WriteProblem(w, envelope.BadRequestProblem.New(envelope.Occurrence{Detail: detail}))
}

// typeErrorMember returns the member of the JSON object body whose value typeErr reports, from decoding body
// into dst. encoding/json names it, except in an error that a field's own UnmarshalJSON returns, such as
// Optional's: then it's the member whose value doesn't decode into its field of dst, a pointer to a struct.
// It reports false if it finds none.
func typeErrorMember(typeErr *json.UnmarshalTypeError, body []byte, dst any) (string, bool) {
	if typeErr.Field != "" {
		return typeErr.Field, true
	}

	t := reflect.TypeOf(dst)
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return "", false
	}

	var members map[string]json.RawMessage
	if err := json.Unmarshal(body, &members); err != nil {
		return "", false
	}

	for field := range t.Elem().Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		value, ok := members[name]
		if !ok {
			continue
		}
		if err := json.Unmarshal(value, reflect.New(field.Type).Interface()); err != nil {
			return name, true
		}
	}

	return "", false
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
