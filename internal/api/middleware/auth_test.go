package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionStoreCreateAndGet(t *testing.T) {
	store := NewSessionStore()

	sess, err := store.Create(1, "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.ID == "" {
		t.Fatal("session ID should not be empty")
	}
	if sess.CSRFToken == "" {
		t.Fatal("CSRF token should not be empty")
	}
	if sess.UserID != 1 {
		t.Fatalf("expected user ID 1, got %d", sess.UserID)
	}
	if sess.Username != "admin" {
		t.Fatalf("expected username admin, got %s", sess.Username)
	}

	got := store.Get(sess.ID)
	if got == nil {
		t.Fatal("expected to find session")
	}
	if got.ID != sess.ID {
		t.Fatalf("session ID mismatch: %s != %s", got.ID, sess.ID)
	}
}

func TestSessionStoreGetExpired(t *testing.T) {
	store := NewSessionStore()

	sess, err := store.Create(1, "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually expire the session.
	store.mu.Lock()
	store.sessions[sess.ID].ExpiresAt = time.Now().Add(-1 * time.Hour)
	store.mu.Unlock()

	got := store.Get(sess.ID)
	if got != nil {
		t.Fatal("expected nil for expired session")
	}
}

func TestSessionStoreGetNotFound(t *testing.T) {
	store := NewSessionStore()
	got := store.Get("nonexistent")
	if got != nil {
		t.Fatal("expected nil for nonexistent session")
	}
}

func TestSessionStoreDelete(t *testing.T) {
	store := NewSessionStore()

	sess, err := store.Create(1, "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store.Delete(sess.ID)

	got := store.Get(sess.ID)
	if got != nil {
		t.Fatal("expected nil after deletion")
	}
}

func TestSessionStoreDeleteByUserID(t *testing.T) {
	store := NewSessionStore()

	s1, _ := store.Create(1, "admin")
	s2, _ := store.Create(1, "admin")
	s3, _ := store.Create(2, "other")

	store.DeleteByUserID(1)

	if store.Get(s1.ID) != nil {
		t.Fatal("session 1 should have been deleted")
	}
	if store.Get(s2.ID) != nil {
		t.Fatal("session 2 should have been deleted")
	}
	if store.Get(s3.ID) == nil {
		t.Fatal("session 3 should still exist")
	}
}

func TestSessionStoreCleanExpired(t *testing.T) {
	store := NewSessionStore()

	s1, _ := store.Create(1, "admin")
	store.Create(2, "other")

	// Expire s1.
	store.mu.Lock()
	store.sessions[s1.ID].ExpiresAt = time.Now().Add(-1 * time.Hour)
	store.mu.Unlock()

	removed := store.CleanExpired()
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
}

func TestRequireAuthNoCookie(t *testing.T) {
	store := NewSessionStore()
	handler := RequireAuth(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuthInvalidSession(t *testing.T) {
	store := NewSessionStore()
	handler := RequireAuth(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRequireAuthValidGetRequest(t *testing.T) {
	store := NewSessionStore()
	sess, _ := store.Create(1, "admin")

	var gotUser *AdminUser
	handler := RequireAuth(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = AdminUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sess.ID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if gotUser == nil {
		t.Fatal("expected admin user in context")
	}
	if gotUser.ID != 1 || gotUser.Username != "admin" {
		t.Fatalf("unexpected user: %+v", gotUser)
	}
}

func TestRequireAuthPostWithoutCSRF(t *testing.T) {
	store := NewSessionStore()
	sess, _ := store.Create(1, "admin")

	handler := RequireAuth(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sess.ID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestRequireAuthPostWithWrongCSRF(t *testing.T) {
	store := NewSessionStore()
	sess, _ := store.Create(1, "admin")

	handler := RequireAuth(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sess.ID})
	req.Header.Set(csrfHeaderName, "wrong-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestRequireAuthPostWithValidCSRF(t *testing.T) {
	store := NewSessionStore()
	sess, _ := store.Create(1, "admin")

	handler := RequireAuth(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sess.ID})
	req.Header.Set(csrfHeaderName, sess.CSRFToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestRequireAuthPutRequiresCSRF(t *testing.T) {
	store := NewSessionStore()
	sess, _ := store.Create(1, "admin")

	handler := RequireAuth(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Without CSRF → 403.
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sess.ID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for PUT without CSRF, got %d", rr.Code)
	}

	// With CSRF → 200.
	req = httptest.NewRequest(http.MethodPut, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sess.ID})
	req.Header.Set(csrfHeaderName, sess.CSRFToken)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for PUT with CSRF, got %d", rr.Code)
	}
}

func TestRequireAuthDeleteRequiresCSRF(t *testing.T) {
	store := NewSessionStore()
	sess, _ := store.Create(1, "admin")

	handler := RequireAuth(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sess.ID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for DELETE without CSRF, got %d", rr.Code)
	}
}

func TestSetSessionCookie(t *testing.T) {
	store := NewSessionStore()
	sess, _ := store.Create(1, "admin")

	rr := httptest.NewRecorder()
	SetSessionCookie(rr, sess, false)

	cookies := rr.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	var sessionCookie, csrfCookie *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case sessionCookieName:
			sessionCookie = c
		case csrfCookieName:
			csrfCookie = c
		}
	}

	if sessionCookie == nil {
		t.Fatal("missing session cookie")
	}
	if sessionCookie.Value != sess.ID {
		t.Fatalf("session cookie value mismatch")
	}
	if !sessionCookie.HttpOnly {
		t.Fatal("session cookie should be HttpOnly")
	}
	if sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("session cookie should have SameSite=Strict")
	}

	if csrfCookie == nil {
		t.Fatal("missing CSRF cookie")
	}
	if csrfCookie.Value != sess.CSRFToken {
		t.Fatalf("CSRF cookie value mismatch")
	}
	if csrfCookie.HttpOnly {
		t.Fatal("CSRF cookie should NOT be HttpOnly")
	}
}

func TestClearSessionCookie(t *testing.T) {
	rr := httptest.NewRecorder()
	ClearSessionCookie(rr, false)

	cookies := rr.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	for _, c := range cookies {
		if c.MaxAge != -1 {
			t.Fatalf("expected MaxAge -1 for %s, got %d", c.Name, c.MaxAge)
		}
	}
}

func TestAdminUserFromContextNil(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	user := AdminUserFromContext(req.Context())
	if user != nil {
		t.Fatal("expected nil user from empty context")
	}
}

func TestSessionIDFromContext(t *testing.T) {
	store := NewSessionStore()
	sess, _ := store.Create(1, "admin")

	var gotSessionID string
	handler := RequireAuth(store, false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSessionID = SessionIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sess.ID})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if gotSessionID != sess.ID {
		t.Fatalf("expected session ID %s, got %s", sess.ID, gotSessionID)
	}
}
