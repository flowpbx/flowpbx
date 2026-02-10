package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// envelope is the standard API response wrapper.
// All JSON responses use this format: { "data": ..., "error": ... }
type envelope struct {
	Data  any    `json:"data"`
	Error string `json:"error,omitempty"`
}

// maxRequestBodySize is the upper limit for JSON request bodies (1 MB).
const maxRequestBodySize = 1 << 20

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

// readJSON decodes a JSON request body into dst. It enforces a size limit,
// rejects unknown fields, and returns a user-friendly error string on failure.
// Returns "" on success.
func readJSON(r *http.Request, dst any) string {
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodySize)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalErr *json.UnmarshalTypeError
		var maxBytesErr *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxErr):
			return "malformed json"
		case errors.As(err, &unmarshalErr):
			if unmarshalErr.Field != "" {
				return "invalid value for field " + unmarshalErr.Field
			}
			return "invalid json value"
		case errors.Is(err, io.EOF):
			return "request body must not be empty"
		case errors.As(err, &maxBytesErr):
			return "request body too large"
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			field := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return "unknown field " + field
		default:
			return "invalid request body"
		}
	}

	// Reject requests that contain more than one JSON value.
	if dec.More() {
		return "request body must contain a single json object"
	}

	return ""
}
