// Package envelope defines the uniform response formats of the API: resource and collection
// envelopes and RFC 9457 Problem Details, plus a writer for bodies with a format of their own.
package envelope

import (
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
)

// jsonMediaType is the media type of every successful JSON response.
const jsonMediaType = "application/json"

// WriteJSON writes v as a JSON response with the given status, without an envelope.
// It is for bodies with a format of their own, such as health checks.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	writeJSON(w, status, jsonMediaType, v)
}

func writeJSON(w http.ResponseWriter, status int, contentType string, v any) {
	writeBody(w, status, contentType, marshalJSON(v))
}

// marshalJSON encodes v before anything is written, so a value that can't be encoded, a bug in
// the caller, panics while the recoverer can still answer with a 500.
func marshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Errorf("envelope: encode response: %w", err))
	}

	return append(b, '\n')
}

func writeBody(w http.ResponseWriter, status int, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	// A failed write only means the client went away; the request logger already logs the request.
	_, _ = w.Write(body)
}

// encodeEnvelope encodes an envelope object whose data member is data, a resource representation of type
// repr or a slice of them, followed by the members of rest in order. It leaves out the zero members of
// each representation, its hidden fields (see Field), but keeps those of rest, such as an offset of 0. It
// fails for a representation with a member that a zero value would leave out too (see checkRepresentation).
func encodeEnvelope(enc *jsontext.Encoder, repr reflect.Type, data any, rest ...member) error {
	if err := checkRepresentation(repr); err != nil {
		return err
	}
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return fmt.Errorf("encode envelope: %w", err)
	}
	if err := encodeMember(enc, member{name: "data", value: data}, jsonv2.OmitZeroStructFields(true)); err != nil {
		return err
	}
	for _, m := range rest {
		if err := encodeMember(enc, m); err != nil {
			return err
		}
	}
	if err := enc.WriteToken(jsontext.EndObject); err != nil {
		return fmt.Errorf("encode envelope: %w", err)
	}

	return nil
}

// member is a member of an envelope object.
type member struct {
	name  string
	value any
}

func encodeMember(enc *jsontext.Encoder, m member, opts ...jsonv2.Options) error {
	if err := enc.WriteToken(jsontext.String(m.name)); err != nil {
		return fmt.Errorf("encode envelope member %s: %w", m.name, err)
	}
	if err := jsonv2.MarshalEncode(enc, m.value, opts...); err != nil {
		return fmt.Errorf("encode envelope member %s: %w", m.name, err)
	}

	return nil
}

// fieldMarker is implemented by every Field.
type fieldMarker interface{ isField() }

// checkedRepresentations caches the result of checkRepresentation by type, an error or nil.
var checkedRepresentations sync.Map

// checkRepresentation returns an error unless repr, the type of a resource representation, is a struct
// whose members are all Fields but its id, which is never zero: since the envelope leaves out every zero
// member to leave out the hidden fields, any other member would vanish when it's zero, such as a false
// or 0. The members of an embedded struct count as members of repr.
func checkRepresentation(repr reflect.Type) error {
	if result, ok := checkedRepresentations.Load(repr); ok {
		err, _ := result.(error)

		return err
	}
	err := checkMembers(repr, repr)
	checkedRepresentations.Store(repr, err)

	return err
}

func checkMembers(repr, t reflect.Type) error {
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("envelope: resource representation %s is a %s, not a struct", repr, t.Kind())
	}
	for member := range t.Fields() {
		if err := checkMember(repr, member); err != nil {
			return err
		}
	}

	return nil
}

func checkMember(repr reflect.Type, member reflect.StructField) error {
	name, _, _ := strings.Cut(member.Tag.Get("json"), ",")
	switch {
	case name == "-" || (!member.IsExported() && !member.Anonymous):
		// Not encoded.
		return nil
	case member.Anonymous && name == "":
		return checkMembers(repr, member.Type)
	case name == "id" || member.Type.Implements(reflect.TypeFor[fieldMarker]()):
		return nil
	default:
		return fmt.Errorf("envelope: member %s of resource representation %s is a %s, not an envelope.Field, "+
			"so it would be left out when zero", member.Name, repr, member.Type)
	}
}
