package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type contextKey string

const (
	// sessionCookieName is the name of the session cookie.
	sessionCookieName = "flowpbx_session"

	// csrfHeaderName is the header that must contain the CSRF token.
	csrfHeaderName = "X-CSRF-Token"

	// csrfCookieName is the cookie that holds the CSRF token for the SPA to read.
	csrfCookieName = "flowpbx_csrf"

	// sessionTTL is the default session lifetime.
	sessionTTL = 24 * time.Hour

	// adminUserKey is the context key for the authenticated admin user.
	adminUserKey contextKey = "admin_user"

	// sessionIDKey is the context key for the session ID.
	sessionIDKey contextKey = "session_id"
)

// AdminUser represents the authenticated admin user stored in the request context.
type AdminUser struct {
	ID       int64
	Username string
}

// Session represents an active admin session.
type Session struct {
	ID        string
	UserID    int64
	Username  string
	CSRFToken string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// SessionStore manages in-memory admin sessions.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionStore creates an empty session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
	}
}

// Create generates a new session for the given admin user and returns it.
func (s *SessionStore) Create(userID int64, username string) (*Session, error) {
	sessionID, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	csrfToken, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	sess := &Session{
		ID:        sessionID,
		UserID:    userID,
		Username:  username,
		CSRFToken: csrfToken,
		ExpiresAt: time.Now().Add(sessionTTL),
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.sessions[sessionID] = sess
	s.mu.Unlock()

	return sess, nil
}

// Get retrieves a session by ID. Returns nil if not found or expired.
func (s *SessionStore) Get(sessionID string) *Session {
	s.mu.RLock()
	sess, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok {
		return nil
	}

	if time.Now().After(sess.ExpiresAt) {
		s.Delete(sessionID)
		return nil
	}

	return sess
}

// Delete removes a session by ID.
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// DeleteByUserID removes all sessions for a given user.
func (s *SessionStore) DeleteByUserID(userID int64) {
	s.mu.Lock()
	for id, sess := range s.sessions {
		if sess.UserID == userID {
			delete(s.sessions, id)
		}
	}
	s.mu.Unlock()
}

// CleanExpired removes all expired sessions.
func (s *SessionStore) CleanExpired() int {
	now := time.Now()
	removed := 0

	s.mu.Lock()
	for id, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			delete(s.sessions, id)
			removed++
		}
	}
	s.mu.Unlock()

	return removed
}

// RequireAuth returns middleware that validates the session cookie and CSRF
// token on state-changing requests. On success it sets the AdminUser in the
// request context. On failure it writes a 401 JSON error.
func RequireAuth(store *SessionStore, secureCookie bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				writeAuthError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			sess := store.Get(cookie.Value)
			if sess == nil {
				writeAuthError(w, http.StatusUnauthorized, "session expired or invalid")
				return
			}

			// CSRF validation for state-changing methods.
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete || r.Method == http.MethodPatch {
				csrfHeader := r.Header.Get(csrfHeaderName)
				if csrfHeader == "" || csrfHeader != sess.CSRFToken {
					writeAuthError(w, http.StatusForbidden, "invalid or missing CSRF token")
					return
				}
			}

			// Store admin user in context for downstream handlers.
			ctx := context.WithValue(r.Context(), adminUserKey, &AdminUser{
				ID:       sess.UserID,
				Username: sess.Username,
			})
			ctx = context.WithValue(ctx, sessionIDKey, sess.ID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SetSessionCookie writes the session and CSRF cookies on the response.
func SetSessionCookie(w http.ResponseWriter, sess *Session, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})

	// CSRF token in a non-HttpOnly cookie so the SPA can read it.
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    sess.CSRFToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

// ClearSessionCookie removes the session and CSRF cookies.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// AdminUserFromContext retrieves the authenticated AdminUser from the context.
// Returns nil if no user is present (unauthenticated request).
func AdminUserFromContext(ctx context.Context) *AdminUser {
	u, _ := ctx.Value(adminUserKey).(*AdminUser)
	return u
}

// SessionIDFromContext retrieves the session ID from the context.
func SessionIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(sessionIDKey).(string)
	return id
}

// StartCleanupTicker runs a goroutine that periodically removes expired sessions.
// It stops when the provided context is cancelled.
func StartCleanupTicker(ctx context.Context, store *SessionStore, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				removed := store.CleanExpired()
				if removed > 0 {
					slog.Debug("cleaned expired sessions", "removed", removed)
				}
			}
		}
	}()
}

// generateToken creates a cryptographically random hex string of the given byte length.
func generateToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// writeAuthError writes a JSON error matching the API envelope format.
// This avoids importing the api package (which would create a circular dependency).
func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Manually format to match envelope: { "error": "..." }
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
