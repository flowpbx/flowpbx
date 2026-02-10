package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// wrapResponseWriter wraps http.ResponseWriter to capture the status code.
type wrapResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func newWrapResponseWriter(w http.ResponseWriter) *wrapResponseWriter {
	return &wrapResponseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (w *wrapResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// StructuredLogger returns middleware that logs each request using log/slog.
// It captures request ID (set by chi's RequestID middleware), HTTP method,
// path, response status, and duration.
func StructuredLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := newWrapResponseWriter(w)

		next.ServeHTTP(wrapped, r)

		reqID := chimw.GetReqID(r.Context())
		duration := time.Since(start)

		slog.Info("http request",
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}
