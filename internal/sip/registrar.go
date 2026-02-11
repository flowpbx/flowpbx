package sip

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
)

const (
	defaultExpiry       = 3600  // 1 hour default registration expiry
	minExpiry           = 60    // 1 minute minimum
	maxExpiry           = 86400 // 24 hours maximum
	expiryCleanupPeriod = 30 * time.Second
)

// Registrar handles SIP REGISTER requests — authenticates, stores contacts
// in the registrations table, and manages expiry cleanup.
type Registrar struct {
	extensions    database.ExtensionRepository
	registrations database.RegistrationRepository
	pushTokens    database.PushTokenRepository
	auth          *Authenticator
	regNotifier   *RegistrationNotifier
	logger        *slog.Logger
}

// NewRegistrar creates a new REGISTER handler.
func NewRegistrar(
	extensions database.ExtensionRepository,
	registrations database.RegistrationRepository,
	pushTokens database.PushTokenRepository,
	auth *Authenticator,
	regNotifier *RegistrationNotifier,
	logger *slog.Logger,
) *Registrar {
	return &Registrar{
		extensions:    extensions,
		registrations: registrations,
		pushTokens:    pushTokens,
		auth:          auth,
		regNotifier:   regNotifier,
		logger:        logger.With("subsystem", "registrar"),
	}
}

// HandleRegister processes incoming REGISTER requests.
func (r *Registrar) HandleRegister(req *sip.Request, tx sip.ServerTransaction) {
	r.logger.Debug("register request received",
		"from", req.From().Address.User,
		"source", req.Source(),
		"method", req.Method,
	)

	// Authenticate the request. Returns nil if auth is pending/failed.
	ext := r.auth.Authenticate(req, tx)
	if ext == nil {
		return
	}

	// Parse the Contact header.
	contact := req.Contact()
	if contact == nil {
		r.logger.Warn("register missing contact header",
			"extension", ext.Extension,
			"source", req.Source(),
		)
		r.respondError(req, tx, 400, "Bad Request")
		return
	}

	// Determine expiry time.
	expiry := r.parseExpiry(req)

	// Handle un-register (Expires: 0 or Contact: *).
	if expiry == 0 || contact.Address.Wildcard {
		r.handleUnregister(req, tx, ext, contact)
		return
	}

	// Clamp expiry to acceptable range.
	if expiry < minExpiry {
		expiry = minExpiry
	}
	if expiry > maxExpiry {
		expiry = maxExpiry
	}

	// Check registration limit.
	ctx := context.Background()
	count, err := r.registrations.CountByExtensionID(ctx, ext.ID)
	if err != nil {
		r.logger.Error("failed to count registrations",
			"extension", ext.Extension,
			"error", err,
		)
		r.respondError(req, tx, 500, "Internal Server Error")
		return
	}

	contactURI := contact.Address.String()

	// Delete existing registration from same contact (re-register).
	if err := r.registrations.DeleteByExtensionAndContact(ctx, ext.ID, contactURI); err != nil {
		r.logger.Error("failed to delete existing registration",
			"extension", ext.Extension,
			"error", err,
		)
		r.respondError(req, tx, 500, "Internal Server Error")
		return
	}

	// Re-check count after removing duplicate contact.
	count, err = r.registrations.CountByExtensionID(ctx, ext.ID)
	if err != nil {
		r.logger.Error("failed to count registrations",
			"extension", ext.Extension,
			"error", err,
		)
		r.respondError(req, tx, 500, "Internal Server Error")
		return
	}

	if int(count) >= ext.MaxRegistrations {
		r.logger.Warn("max registrations exceeded",
			"extension", ext.Extension,
			"current", count,
			"max", ext.MaxRegistrations,
		)
		r.respondError(req, tx, 403, "Forbidden")
		return
	}

	// Parse source address.
	sourceIP, sourcePort := r.parseSource(req)

	// Extract push token and device_id from Contact parameters.
	pushToken, pushPlatform, deviceID := r.parsePushParams(contact)

	// Parse transport from Via header.
	transport := r.parseTransport(req)

	// Parse User-Agent.
	userAgent := ""
	if ua := req.GetHeader("User-Agent"); ua != nil {
		userAgent = ua.Value()
	}

	// Store the registration.
	reg := &models.Registration{
		ExtensionID:  &ext.ID,
		ContactURI:   contactURI,
		Transport:    transport,
		UserAgent:    userAgent,
		SourceIP:     sourceIP,
		SourcePort:   sourcePort,
		Expires:      time.Now().Add(time.Duration(expiry) * time.Second),
		PushToken:    pushToken,
		PushPlatform: pushPlatform,
		DeviceID:     deviceID,
	}

	if err := r.registrations.Create(ctx, reg); err != nil {
		r.logger.Error("failed to store registration",
			"extension", ext.Extension,
			"error", err,
		)
		r.respondError(req, tx, 500, "Internal Server Error")
		return
	}

	// Persist push token in the dedicated push_tokens table so it survives
	// registration expiry. The mobile app can still receive push wake-ups
	// even after the SIP registration is cleaned up.
	if pushToken != "" && pushPlatform != "" && deviceID != "" && r.pushTokens != nil {
		pt := &models.PushToken{
			ExtensionID: ext.ID,
			Token:       pushToken,
			Platform:    pushPlatform,
			DeviceID:    deviceID,
			AppVersion:  userAgent,
		}
		if err := r.pushTokens.Upsert(ctx, pt); err != nil {
			r.logger.Error("failed to upsert push token",
				"extension", ext.Extension,
				"device_id", deviceID,
				"error", err,
			)
			// Non-fatal — the registration itself succeeded.
		}
	}

	r.logger.Info("extension registered",
		"extension", ext.Extension,
		"contact", contactURI,
		"transport", transport,
		"expires", expiry,
		"source", req.Source(),
		"push_token_present", pushToken != "",
	)

	// Notify any push-wait callers that this extension has registered.
	if r.regNotifier != nil {
		r.regNotifier.Notify(ext.ID)
	}

	// Send 200 OK with the registered Contact and Expires.
	res := sip.NewResponseFromRequest(req, 200, "OK", nil)
	res.AppendHeader(&sip.ContactHeader{
		Address: contact.Address,
	})
	res.AppendHeader(sip.NewHeader("Expires", strconv.Itoa(expiry)))

	if err := tx.Respond(res); err != nil {
		r.logger.Error("failed to send register response", "error", err)
	}
}

// handleUnregister processes un-registration (Expires: 0).
func (r *Registrar) handleUnregister(req *sip.Request, tx sip.ServerTransaction, ext *models.Extension, contact *sip.ContactHeader) {
	ctx := context.Background()

	if contact.Address.Wildcard {
		// Contact: * — remove all registrations for this extension.
		regs, err := r.registrations.GetByExtensionID(ctx, ext.ID)
		if err != nil {
			r.logger.Error("failed to get registrations for unregister",
				"extension", ext.Extension,
				"error", err,
			)
			r.respondError(req, tx, 500, "Internal Server Error")
			return
		}
		for _, reg := range regs {
			if err := r.registrations.DeleteByID(ctx, reg.ID); err != nil {
				r.logger.Error("failed to delete registration",
					"id", reg.ID,
					"error", err,
				)
			}
		}
		r.logger.Info("all registrations removed",
			"extension", ext.Extension,
			"count", len(regs),
		)
	} else {
		// Remove specific contact registration.
		contactURI := contact.Address.String()
		if err := r.registrations.DeleteByExtensionAndContact(ctx, ext.ID, contactURI); err != nil {
			r.logger.Error("failed to delete registration",
				"extension", ext.Extension,
				"contact", contactURI,
				"error", err,
			)
			r.respondError(req, tx, 500, "Internal Server Error")
			return
		}
		r.logger.Info("registration removed",
			"extension", ext.Extension,
			"contact", contactURI,
		)
	}

	res := sip.NewResponseFromRequest(req, 200, "OK", nil)
	if err := tx.Respond(res); err != nil {
		r.logger.Error("failed to send unregister response", "error", err)
	}
}

// RunExpiryCleanup periodically removes expired registrations.
func (r *Registrar) RunExpiryCleanup(ctx context.Context) {
	ticker := time.NewTicker(expiryCleanupPeriod)
	defer ticker.Stop()

	r.logger.Info("registration expiry cleanup started",
		"interval", expiryCleanupPeriod.String(),
	)

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("registration expiry cleanup stopped")
			return
		case <-ticker.C:
			deleted, err := r.registrations.DeleteExpired(ctx)
			if err != nil {
				r.logger.Error("failed to clean expired registrations", "error", err)
				continue
			}
			if deleted > 0 {
				r.logger.Info("expired registrations cleaned", "count", deleted)
			}

			// Also clean expired nonces from the authenticator.
			r.auth.CleanExpiredNonces()
		}
	}
}

// parseExpiry extracts the registration expiry from the request.
// Checks Contact params first, then Expires header, then uses default.
func (r *Registrar) parseExpiry(req *sip.Request) int {
	// Check Contact header expires parameter.
	if contact := req.Contact(); contact != nil {
		if val, ok := contact.Params.Get("expires"); ok {
			if exp, err := strconv.Atoi(val); err == nil {
				return exp
			}
		}
	}

	// Check Expires header.
	if h := req.GetHeader("Expires"); h != nil {
		if exp, err := strconv.Atoi(h.Value()); err == nil {
			return exp
		}
	}

	return defaultExpiry
}

// parseSource extracts the source IP and port from the request.
func (r *Registrar) parseSource(req *sip.Request) (string, int) {
	source := req.Source()
	host, portStr, err := net.SplitHostPort(source)
	if err != nil {
		return source, 0
	}
	port, _ := strconv.Atoi(portStr)
	return host, port
}

// parsePushParams extracts push notification parameters from Contact header.
// Convention: Contact params +sip.pnsprov, +sip.pnsreg (push token), +sip.pnspurr (device_id)
// Also supports simpler params: pn-tok, pn-type, pn-device.
func (r *Registrar) parsePushParams(contact *sip.ContactHeader) (token, platform, deviceID string) {
	if contact == nil {
		return "", "", ""
	}

	// Standard-ish push notification parameters (RFC 8599 inspired).
	if tok, ok := contact.Params.Get("pn-tok"); ok {
		token = tok
	}
	if typ, ok := contact.Params.Get("pn-type"); ok {
		platform = typ
	}
	if dev, ok := contact.Params.Get("pn-device"); ok {
		deviceID = dev
	}

	// Also check URI parameters.
	if token == "" {
		if tok, ok := contact.Address.UriParams.Get("pn-tok"); ok {
			token = tok
		}
	}
	if platform == "" {
		if typ, ok := contact.Address.UriParams.Get("pn-type"); ok {
			platform = typ
		}
	}
	if deviceID == "" {
		if dev, ok := contact.Address.UriParams.Get("pn-device"); ok {
			deviceID = dev
		}
	}

	return token, platform, deviceID
}

// parseTransport determines the transport protocol from the Via header.
func (r *Registrar) parseTransport(req *sip.Request) string {
	if via := req.Via(); via != nil {
		transport := strings.ToLower(via.Transport)
		if transport != "" {
			return transport
		}
	}
	return "udp"
}

func (r *Registrar) respondError(req *sip.Request, tx sip.ServerTransaction, code int, reason string) {
	res := sip.NewResponseFromRequest(req, code, reason, nil)
	if err := tx.Respond(res); err != nil {
		r.logger.Error("failed to send error response",
			"code", code,
			"error", err,
		)
	}
}
