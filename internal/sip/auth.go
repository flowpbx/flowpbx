package sip

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/icholy/digest"
)

const (
	authRealm   = "flowpbx"
	nonceExpiry = 5 * time.Minute
	authAlgoMD5 = "MD5"
)

// Authenticator handles SIP digest authentication against the extensions table.
// It integrates with BruteForceGuard to automatically block source IPs that
// exceed the failed authentication threshold (fail2ban-style protection).
type Authenticator struct {
	extensions database.ExtensionRepository
	encryptor  *database.Encryptor
	logger     *slog.Logger
	nonces     sync.Map // map[string]time.Time — tracks issued nonces
	guard      *BruteForceGuard
}

// NewAuthenticator creates a new SIP digest authenticator with brute-force
// protection enabled. The encryptor is optional — if nil, SIP passwords are
// assumed to be stored in plaintext.
func NewAuthenticator(extensions database.ExtensionRepository, enc *database.Encryptor, logger *slog.Logger) *Authenticator {
	return &Authenticator{
		extensions: extensions,
		encryptor:  enc,
		logger:     logger.With("subsystem", "auth"),
		guard:      NewBruteForceGuard(logger),
	}
}

// Challenge sends a 401 Unauthorized response with a WWW-Authenticate header.
func (a *Authenticator) Challenge(req *sip.Request, tx sip.ServerTransaction) {
	nonce := a.generateNonce()
	a.nonces.Store(nonce, time.Now())

	chal := digest.Challenge{
		Realm:     authRealm,
		Nonce:     nonce,
		Opaque:    "flowpbx",
		Algorithm: authAlgoMD5,
	}

	res := sip.NewResponseFromRequest(req, 401, "Unauthorized", nil)
	res.AppendHeader(sip.NewHeader("WWW-Authenticate", chal.String()))

	if err := tx.Respond(res); err != nil {
		a.logger.Error("failed to send auth challenge", "error", err)
	}
}

// Authenticate validates the Authorization header against the extensions table.
// Returns the matched extension on success, or nil if authentication fails.
// When authentication fails, it sends the appropriate SIP error response.
//
// Brute-force protection: if the source IP is blocked by the BruteForceGuard,
// the request is rejected with 403 Forbidden without processing credentials.
func (a *Authenticator) Authenticate(req *sip.Request, tx sip.ServerTransaction) *models.Extension {
	source := req.Source()

	// Check brute-force guard before processing any credentials.
	if a.guard.IsBlocked(source) {
		a.logger.Warn("sip auth rejected: ip blocked by brute-force guard",
			"source", source,
		)
		a.respondError(req, tx, 403, "Forbidden")
		return nil
	}

	h := req.GetHeader("Authorization")
	if h == nil {
		a.Challenge(req, tx)
		return nil
	}

	cred, err := digest.ParseCredentials(h.Value())
	if err != nil {
		a.logger.Warn("failed to parse authorization header",
			"error", err,
			"source", source,
		)
		a.guard.RecordFailure(source)
		a.respondError(req, tx, 400, "Bad Request")
		return nil
	}

	// Validate nonce to prevent replay attacks.
	nonceTime, ok := a.nonces.Load(cred.Nonce)
	if !ok {
		a.logger.Debug("unknown nonce, re-challenging",
			"username", cred.Username,
			"source", source,
		)
		a.Challenge(req, tx)
		return nil
	}
	if time.Since(nonceTime.(time.Time)) > nonceExpiry {
		a.nonces.Delete(cred.Nonce)
		a.logger.Debug("expired nonce, re-challenging",
			"username", cred.Username,
			"source", source,
		)
		a.Challenge(req, tx)
		return nil
	}

	// Look up extension by SIP username.
	ext, err := a.extensions.GetBySIPUsername(context.Background(), cred.Username)
	if err != nil {
		a.logger.Error("failed to look up extension",
			"username", cred.Username,
			"error", err,
		)
		a.respondError(req, tx, 500, "Internal Server Error")
		return nil
	}
	if ext == nil {
		a.logger.Warn("unknown sip username",
			"username", cred.Username,
			"source", source,
		)
		a.guard.RecordFailure(source)
		a.respondError(req, tx, 403, "Forbidden")
		return nil
	}

	// Reconstruct the challenge to verify the digest response.
	chal := digest.Challenge{
		Realm:     authRealm,
		Nonce:     cred.Nonce,
		Opaque:    "flowpbx",
		Algorithm: authAlgoMD5,
	}

	// Decrypt SIP password if encrypted at rest.
	sipPassword := ext.SIPPassword
	if a.encryptor != nil && sipPassword != "" {
		decrypted, err := a.encryptor.Decrypt(sipPassword)
		if err != nil {
			a.logger.Error("failed to decrypt sip password for digest auth",
				"username", cred.Username,
				"error", err,
			)
			a.respondError(req, tx, 500, "Internal Server Error")
			return nil
		}
		sipPassword = decrypted
	}

	expected, err := digest.Digest(&chal, digest.Options{
		Method:   string(req.Method),
		URI:      cred.URI,
		Username: cred.Username,
		Password: sipPassword,
	})
	if err != nil {
		a.logger.Error("failed to compute digest",
			"username", cred.Username,
			"error", err,
		)
		a.respondError(req, tx, 500, "Internal Server Error")
		return nil
	}

	if cred.Response != expected.Response {
		a.logger.Warn("digest auth failed",
			"username", cred.Username,
			"source", source,
		)
		a.guard.RecordFailure(source)
		a.Challenge(req, tx)
		return nil
	}

	// Consume the nonce after successful auth.
	a.nonces.Delete(cred.Nonce)

	// Clear failure counter on successful auth.
	a.guard.RecordSuccess(source)

	a.logger.Debug("digest auth successful",
		"username", cred.Username,
		"extension", ext.Extension,
	)
	return ext
}

// CleanExpiredNonces removes nonces that are older than the expiry window
// and runs brute-force guard cleanup to expire old blocks.
func (a *Authenticator) CleanExpiredNonces() {
	now := time.Now()
	a.nonces.Range(func(key, value any) bool {
		if now.Sub(value.(time.Time)) > nonceExpiry {
			a.nonces.Delete(key)
		}
		return true
	})
	a.guard.Cleanup()
}

// BruteForceGuard returns the brute-force guard for admin visibility
// (listing blocked IPs, manual unblock).
func (a *Authenticator) BruteForceGuard() *BruteForceGuard {
	return a.guard
}

func (a *Authenticator) generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based nonce.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func (a *Authenticator) respondError(req *sip.Request, tx sip.ServerTransaction, code int, reason string) {
	res := sip.NewResponseFromRequest(req, code, reason, nil)
	if err := tx.Respond(res); err != nil {
		a.logger.Error("failed to send error response",
			"code", code,
			"error", err,
		)
	}
}
