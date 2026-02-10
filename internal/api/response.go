package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
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

// defaultLimit is the default number of items per page when not specified.
const defaultLimit = 20

// maxLimit is the maximum allowed limit to prevent excessive result sets.
const maxLimit = 100

// Pagination holds parsed limit/offset query parameters for list endpoints.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// PaginatedResponse wraps a list of items with pagination metadata.
type PaginatedResponse struct {
	Items  any `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// parsePagination extracts limit and offset from query parameters with
// validation and sensible defaults. Returns an error string (empty on success)
// following the same pattern as readJSON.
func parsePagination(r *http.Request) (Pagination, string) {
	q := r.URL.Query()

	limit := defaultLimit
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Pagination{}, "limit must be a positive integer"
		}
		if n > maxLimit {
			n = maxLimit
		}
		limit = n
	}

	offset := 0
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return Pagination{}, "offset must be a non-negative integer"
		}
		offset = n
	}

	return Pagination{Limit: limit, Offset: offset}, ""
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
