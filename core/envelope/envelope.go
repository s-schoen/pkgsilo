// Package envelope defines the uniform response formats of the API: resource and collection
// envelopes and RFC 9457 Problem Details, plus a writer for bodies with a format of their own.
package envelope

import (
	"encoding/json"
	"fmt"
	"net/http"
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
