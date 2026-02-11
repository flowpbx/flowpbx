package middleware

import "net/http"

// SecurityHeaders returns middleware that sets HTTP security headers on every
// response. When tlsEnabled is true, Strict-Transport-Security (HSTS) is
// included; it is omitted on plain HTTP to avoid browsers caching an HSTS
// policy for a host that does not support TLS.
func SecurityHeaders(tlsEnabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()

			// Prevent clickjacking.
			h.Set("X-Frame-Options", "DENY")

			// Prevent MIME type sniffing.
			h.Set("X-Content-Type-Options", "nosniff")

			// Disable legacy XSS filter — CSP supersedes it and the old
			// filter can introduce vulnerabilities.
			h.Set("X-XSS-Protection", "0")

			// Limit referrer information leaked to other origins.
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Content Security Policy: allow resources from the same origin
			// only. 'unsafe-inline' is needed for Vite-bundled styles and
			// inline scripts injected during the React build. connect-src
			// includes ws:/wss: for the live-stats WebSocket.
			h.Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data:; "+
					"font-src 'self'; "+
					"connect-src 'self' ws: wss:; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'; "+
					"form-action 'self'")

			// Restrict access to powerful browser features.
			h.Set("Permissions-Policy",
				"camera=(), microphone=(), geolocation=(), payment=()")

			// HSTS — only sent when serving over TLS.
			if tlsEnabled {
				// max-age=63072000 is 2 years; includeSubDomains ensures
				// all subdomains also require HTTPS.
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}
