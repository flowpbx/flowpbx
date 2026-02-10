package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowedOriginSetsHeaders(t *testing.T) {
	handler := CORS([]string{"https://admin.example.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Fatalf("expected origin https://admin.example.com, got %q", got)
	}
	if got := rr.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary: Origin, got %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected Allow-Credentials true, got %q", got)
	}
}

func TestCORSDisallowedOriginNoHeaders(t *testing.T) {
	handler := CORS([]string{"https://admin.example.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Allow-Origin header, got %q", got)
	}
}

func TestCORSWildcardAllowsAny(t *testing.T) {
	handler := CORS([]string{"*"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "https://anything.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard origin *, got %q", got)
	}
	// Wildcard should not set Vary: Origin.
	if got := rr.Header().Get("Vary"); got != "" {
		t.Fatalf("expected no Vary header for wildcard, got %q", got)
	}
}

func TestCORSPreflightReturns204(t *testing.T) {
	handler := CORS([]string{"https://admin.example.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/extensions", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("expected Allow-Methods header on preflight")
	}
	if got := rr.Header().Get("Access-Control-Max-Age"); got != "300" {
		t.Fatalf("expected Max-Age 300, got %q", got)
	}
}

func TestCORSNoOriginHeaderNoHeaders(t *testing.T) {
	handler := CORS([]string{"https://admin.example.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	// No Origin header set.
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Allow-Origin header without Origin, got %q", got)
	}
}

func TestCORSEmptyOriginsDisablesCORS(t *testing.T) {
	handler := CORS(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS headers with empty origins, got %q", got)
	}
}

func TestCORSMultipleOrigins(t *testing.T) {
	origins := []string{"https://admin.example.com", "https://dev.example.com"}
	handler := CORS(origins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First origin should work.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Fatalf("expected first origin, got %q", got)
	}

	// Second origin should work.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://dev.example.com")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://dev.example.com" {
		t.Fatalf("expected second origin, got %q", got)
	}
}

func TestParseCORSOriginsEmpty(t *testing.T) {
	if got := ParseCORSOrigins(""); got != nil {
		t.Fatalf("expected nil for empty input, got %v", got)
	}
	if got := ParseCORSOrigins("   "); got != nil {
		t.Fatalf("expected nil for whitespace input, got %v", got)
	}
}

func TestParseCORSOriginsSingle(t *testing.T) {
	got := ParseCORSOrigins("https://example.com")
	if len(got) != 1 || got[0] != "https://example.com" {
		t.Fatalf("expected [https://example.com], got %v", got)
	}
}

func TestParseCORSOriginsMultiple(t *testing.T) {
	got := ParseCORSOrigins("https://a.com, https://b.com , https://c.com")
	if len(got) != 3 {
		t.Fatalf("expected 3 origins, got %d: %v", len(got), got)
	}
	if got[0] != "https://a.com" || got[1] != "https://b.com" || got[2] != "https://c.com" {
		t.Fatalf("unexpected origins: %v", got)
	}
}

func TestParseCORSOriginsWildcard(t *testing.T) {
	got := ParseCORSOrigins("*")
	if len(got) != 1 || got[0] != "*" {
		t.Fatalf("expected [*], got %v", got)
	}
}
