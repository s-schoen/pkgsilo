// Package envelope defines the uniform response formats of the API: resource and collection
// envelopes, RFC 9457 Problem Details and health responses.
package envelope

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// jsonMediaType is the media type of every successful JSON response.
const jsonMediaType = "application/json"

func writeJSON(w http.ResponseWriter, status int, contentType string, v any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
