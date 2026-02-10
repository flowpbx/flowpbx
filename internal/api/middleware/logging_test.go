package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStructuredLoggerDefaultStatus(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	slog.SetDefault(logger)

	handler := StructuredLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	if logEntry["method"] != "GET" {
		t.Fatalf("expected method GET, got %v", logEntry["method"])
	}
	if logEntry["path"] != "/api/v1/health" {
		t.Fatalf("expected path /api/v1/health, got %v", logEntry["path"])
	}
	// JSON numbers decode as float64.
	if logEntry["status"] != float64(200) {
		t.Fatalf("expected status 200, got %v", logEntry["status"])
	}
	if _, ok := logEntry["duration_ms"]; !ok {
		t.Fatal("expected duration_ms in log output")
	}
}

func TestStructuredLoggerExplicitStatus(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	slog.SetDefault(logger)

	handler := StructuredLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/missing", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	if logEntry["method"] != "POST" {
		t.Fatalf("expected method POST, got %v", logEntry["method"])
	}
	if logEntry["path"] != "/api/v1/missing" {
		t.Fatalf("expected path /api/v1/missing, got %v", logEntry["path"])
	}
	if logEntry["status"] != float64(404) {
		t.Fatalf("expected status 404, got %v", logEntry["status"])
	}
}

func TestStructuredLoggerDoubleWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	slog.SetDefault(logger)

	handler := StructuredLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusInternalServerError) // Should be ignored.
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	if logEntry["status"] != float64(201) {
		t.Fatalf("expected first status 201, got %v", logEntry["status"])
	}
}

func TestWrapResponseWriterDefaultStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	w := newWrapResponseWriter(rr)

	if w.status != http.StatusOK {
		t.Fatalf("expected default status 200, got %d", w.status)
	}
}

func TestWrapResponseWriterCapturesStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	w := newWrapResponseWriter(rr)

	w.WriteHeader(http.StatusBadRequest)

	if w.status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.status)
	}
}
