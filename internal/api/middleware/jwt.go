package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// appExtensionKey is the context key for the authenticated app extension.
type appContextKey string

const appExtensionIDKey appContextKey = "app_extension_id"

// jwtTokenTTL is the lifetime of an app JWT token (7 days).
const jwtTokenTTL = 7 * 24 * time.Hour

// AppClaims holds the JWT claims for mobile app authentication.
type AppClaims struct {
	ExtensionID int64  `json:"ext_id"`
	Extension   string `json:"ext"`
	jwt.RegisteredClaims
}

// GenerateAppToken creates a signed JWT for a mobile app extension login.
func GenerateAppToken(secret []byte, extensionID int64, extension string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(jwtTokenTTL)

	claims := AppClaims{
		ExtensionID: extensionID,
		Extension:   extension,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    "flowpbx",
			Subject:   extension,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return signed, expiresAt, nil
}

// RequireAppAuth returns middleware that validates JWT bearer tokens for mobile
// app endpoints. On success it stores the extension ID in the request context.
func RequireAppAuth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJWTError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				writeJWTError(w, http.StatusUnauthorized, "invalid authorization header")
				return
			}

			tokenString := parts[1]

			claims := &AppClaims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return secret, nil
			})
			if err != nil || !token.Valid {
				slog.Debug("app auth: invalid jwt", "error", err)
				writeJWTError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			if claims.ExtensionID == 0 {
				writeJWTError(w, http.StatusUnauthorized, "invalid token claims")
				return
			}

			ctx := context.WithValue(r.Context(), appExtensionIDKey, claims.ExtensionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AppExtensionIDFromContext retrieves the authenticated extension ID from the
// request context. Returns 0 if not set.
func AppExtensionIDFromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(appExtensionIDKey).(int64)
	return id
}

// jwtEnvelope matches the api package's envelope format for error responses.
type jwtEnvelope struct {
	Error string `json:"error,omitempty"`
}

// writeJWTError writes a JSON error matching the API envelope format.
func writeJWTError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(jwtEnvelope{Error: msg}) //nolint:errcheck
}
