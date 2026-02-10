package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// Recoverer returns middleware that recovers from panics, logs the stack trace
// using slog, and returns a 500 Internal Server Error JSON response.
// It should be mounted after StructuredLogger so the request ID is available.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				reqID := chimw.GetReqID(r.Context())
				stack := debug.Stack()

				slog.Error("panic recovered",
					"request_id", reqID,
					"panic", rec,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(stack),
				)

				// Avoid writing if the response has already started.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"internal server error"}`))
			}
		}()

		next.ServeHTTP(w, r)
	})
}
