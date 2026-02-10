package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"name": "test"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected content-type application/json, got %q", ct)
	}

	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if env.Error != "" {
		t.Errorf("expected empty error, got %q", env.Error)
	}

	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got %T", env.Data)
	}
	if data["name"] != "test" {
		t.Errorf("expected name=test, got %v", data["name"])
	}
}

func TestWriteJSON_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, nil)

	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if env.Data != nil {
		t.Errorf("expected nil data, got %v", env.Data)
	}
	if env.Error != "" {
		t.Errorf("expected empty error, got %q", env.Error)
	}
}

func TestWriteJSON_CustomStatus(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]int{"id": 1})

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected content-type application/json, got %q", ct)
	}

	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if env.Error != "invalid input" {
		t.Errorf("expected error 'invalid input', got %q", env.Error)
	}
	if env.Data != nil {
		t.Errorf("expected nil data, got %v", env.Data)
	}
}

func TestWriteError_OmitsEmptyError(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, "ok")

	body := w.Body.String()
	if strings.Contains(body, `"error"`) {
		t.Errorf("expected error field to be omitted, got %s", body)
	}
}

func TestReadJSON_Success(t *testing.T) {
	body := strings.NewReader(`{"name":"test","value":42}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var dst struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	errMsg := readJSON(r, &dst)
	if errMsg != "" {
		t.Fatalf("expected no error, got %q", errMsg)
	}
	if dst.Name != "test" {
		t.Errorf("expected name=test, got %q", dst.Name)
	}
	if dst.Value != 42 {
		t.Errorf("expected value=42, got %d", dst.Value)
	}
}

func TestReadJSON_EmptyBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))

	var dst struct{}
	errMsg := readJSON(r, &dst)
	if errMsg != "request body must not be empty" {
		t.Errorf("expected 'request body must not be empty', got %q", errMsg)
	}
}

func TestReadJSON_MalformedJSON(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{bad"))

	var dst struct{}
	errMsg := readJSON(r, &dst)
	if errMsg != "malformed json" {
		t.Errorf("expected 'malformed json', got %q", errMsg)
	}
}

func TestReadJSON_UnknownField(t *testing.T) {
	body := strings.NewReader(`{"name":"test","extra":"field"}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var dst struct {
		Name string `json:"name"`
	}

	errMsg := readJSON(r, &dst)
	if !strings.HasPrefix(errMsg, "unknown field") {
		t.Errorf("expected 'unknown field ...' error, got %q", errMsg)
	}
}

func TestReadJSON_WrongType(t *testing.T) {
	body := strings.NewReader(`{"value":"not_a_number"}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var dst struct {
		Value int `json:"value"`
	}

	errMsg := readJSON(r, &dst)
	if errMsg == "" {
		t.Error("expected error for wrong type, got empty string")
	}
}

func TestReadJSON_MultipleObjects(t *testing.T) {
	body := strings.NewReader(`{"a":1}{"b":2}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var dst struct {
		A int `json:"a"`
	}

	errMsg := readJSON(r, &dst)
	if errMsg != "request body must contain a single json object" {
		t.Errorf("expected single object error, got %q", errMsg)
	}
}

func TestEnvelope_JSONFormat(t *testing.T) {
	// Verify the envelope serializes to the expected format.
	e := envelope{Data: map[string]string{"id": "1"}}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Should have "data" but not "error" (omitempty).
	if !strings.Contains(string(b), `"data"`) {
		t.Error("expected 'data' field in output")
	}
	if strings.Contains(string(b), `"error"`) {
		t.Error("expected 'error' field to be omitted")
	}

	// Now with an error.
	e = envelope{Error: "bad request"}
	b, err = json.Marshal(e)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if !strings.Contains(string(b), `"error":"bad request"`) {
		t.Errorf("expected error field, got %s", string(b))
	}
}
