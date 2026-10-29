// Package envelope defines the uniform response formats of the API: resource and collection
// envelopes, RFC 9457 Problem Details and health responses.
package envelope

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// jsonMediaType is the media type of every successful JSON response.
const jsonMediaType = "application/json"

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
