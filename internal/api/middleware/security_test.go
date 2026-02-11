package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersSetAllHeaders(t *testing.T) {
	handler := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	expected := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"X-XSS-Protection":       "0",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for header, want := range expected {
		if got := rr.Header().Get(header); got != want {
			t.Errorf("%s: expected %q, got %q", header, want, got)
		}
	}

	if got := rr.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("expected Content-Security-Policy header to be set")
	}
	if got := rr.Header().Get("Permissions-Policy"); got == "" {
		t.Error("expected Permissions-Policy header to be set")
	}
}

func TestSecurityHeadersNoHSTSWithoutTLS(t *testing.T) {
	handler := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("expected no HSTS header without TLS, got %q", got)
	}
}

func TestSecurityHeadersHSTSWithTLS(t *testing.T) {
	handler := SecurityHeaders(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	got := rr.Header().Get("Strict-Transport-Security")
	if got == "" {
		t.Fatal("expected HSTS header with TLS enabled")
	}
	if got != "max-age=63072000; includeSubDomains" {
		t.Fatalf("unexpected HSTS value: %q", got)
	}
}

func TestSecurityHeadersCSPContainsFrameAncestors(t *testing.T) {
	handler := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("expected CSP header to be set")
	}

	// Verify key directives are present.
	directives := []string{
		"default-src 'self'",
		"script-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	}
	for _, d := range directives {
		found := false
		for _, part := range splitCSP(csp) {
			if part == d {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("CSP missing directive %q in: %s", d, csp)
		}
	}
}

func TestSecurityHeadersPermissionsPolicy(t *testing.T) {
	handler := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	pp := rr.Header().Get("Permissions-Policy")
	if pp == "" {
		t.Fatal("expected Permissions-Policy header")
	}
	// Ensure dangerous APIs are denied.
	for _, feature := range []string{"camera=()", "microphone=()", "geolocation=()"} {
		if !containsSubstring(pp, feature) {
			t.Errorf("Permissions-Policy missing %q in: %s", feature, pp)
		}
	}
}

func TestSecurityHeadersPassesThroughToHandler(t *testing.T) {
	called := false
	handler := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/extensions", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

// splitCSP splits a CSP header value into individual directives by semicolon.
func splitCSP(csp string) []string {
	var parts []string
	for _, part := range splitString(csp, ";") {
		trimmed := trimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// splitString splits s by sep without importing strings (test helper).
func splitString(s, sep string) []string {
	var parts []string
	for {
		i := indexOf(s, sep)
		if i < 0 {
			parts = append(parts, s)
			break
		}
		parts = append(parts, s[:i])
		s = s[i+len(sep):]
	}
	return parts
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func containsSubstring(s, sub string) bool {
	return indexOf(s, sub) >= 0
}
