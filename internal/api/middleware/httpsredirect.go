package middleware

import (
	"net/http"
)

// HTTPSRedirectHandler returns an http.Handler that redirects all HTTP requests
// to the equivalent HTTPS URL with a 301 Moved Permanently status. This is
// intended to run as a separate HTTP server alongside the main HTTPS server.
func HTTPSRedirectHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}
