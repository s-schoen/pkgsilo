package core

import "net/http"

// SecurityHeaders sets restrictive browser security headers suitable for a JSON API.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// Stop browsers from MIME-sniffing responses into something executable.
		h.Set("X-Content-Type-Options", "nosniff")
		// Forbid framing (legacy browsers; frame-ancestors covers modern ones).
		h.Set("X-Frame-Options", "DENY")
		// API responses never need to load or run anything.
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		// Block Adobe Flash/PDF cross-domain policy files.
		h.Set("X-Permitted-Cross-Domain-Policies", "none")

		next.ServeHTTP(w, r)
	})
}
