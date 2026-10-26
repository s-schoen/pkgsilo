package middleware

import (
	"context"
	"net/http"
	"regexp"
	"uuid"
)

// RequestIDHeader carries the request's correlation ID, both inbound and outbound.
const RequestIDHeader = "X-Request-ID"

// validRequestID restricts client-supplied IDs to a safe charset and length,
// so they can't bloat or garble log lines.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type requestIDKey struct{}

// RequestID attaches a correlation ID to the request context and echoes it in the response.
// A valid incoming X-Request-ID is reused, otherwise a new UUIDv7 is generated.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !validRequestID.MatchString(id) {
			id = uuid.NewV7().String()
		}

		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

// RequestIDFromContext returns the request's correlation ID, or "" if there is none.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
