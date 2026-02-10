package middleware

import (
	"net/http"
	"strings"
)

// CORS returns middleware that sets Cross-Origin Resource Sharing headers.
// allowedOrigins is a slice of permitted origins. If the slice contains "*",
// all origins are allowed (suitable for development; not recommended for
// production). An empty slice disables CORS entirely — no headers are sent
// and preflight requests receive 204 with no allow headers.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	// Build a lookup set for O(1) origin checks.
	allowAll := false
	origins := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "*" {
			allowAll = true
		}
		if o != "" {
			origins[o] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Only set CORS headers when an Origin header is present and
			// the origin is in the allowed list.
			if origin != "" && (allowAll || origins[origin]) {
				h := w.Header()
				if allowAll {
					h.Set("Access-Control-Allow-Origin", "*")
				} else {
					h.Set("Access-Control-Allow-Origin", origin)
					h.Set("Vary", "Origin")
				}
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Set("Access-Control-Max-Age", "300")
			}

			// Handle preflight requests.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ParseCORSOrigins splits a comma-separated origins string into a slice.
// Empty input returns nil.
func ParseCORSOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			origins = append(origins, p)
		}
	}
	return origins
}
