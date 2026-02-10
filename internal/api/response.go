package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// envelope is the standard API response wrapper.
// All JSON responses use this format: { "data": ..., "error": ... }
type envelope struct {
	Data  any    `json:"data"`
	Error string `json:"error,omitempty"`
}

// writeJSON writes a JSON response with the given status code and data payload.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{Data: data}); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

// writeError writes a JSON error response with the given status code and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{Error: msg}); err != nil {
		slog.Error("failed to encode json error response", "error", err)
	}
}
