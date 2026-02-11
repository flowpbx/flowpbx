package sip

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/flowpbx/flowpbx/internal/database/models"
	"github.com/icholy/digest"
)

// TrunkStatus represents the registration state of a trunk.
type TrunkStatus string

const (
	TrunkStatusRegistered   TrunkStatus = "registered"
	TrunkStatusFailed       TrunkStatus = "failed"
	TrunkStatusDisabled     TrunkStatus = "disabled"
	TrunkStatusUnregistered TrunkStatus = "unregistered"
	TrunkStatusRegistering  TrunkStatus = "registering"
)

// TrunkState holds the runtime state for a single trunk.
type TrunkState struct {
	TrunkID        int64
	Name           string
	Type           string
	Status         TrunkStatus
	LastError      string
	RetryAttempt   int
	FailedAt       *time.Time
	RegisteredAt   *time.Time
	ExpiresAt      *time.Time
	LastOptionsAt  *time.Time
	OptionsHealthy bool
}

// TrunkRegistrar manages outbound SIP registrations for register-type trunks.
// It sends REGISTER requests to upstream providers and maintains registration
// state with periodic re-registration. It also manages IP-auth trunk ACL
// matching for inbound call identification.
type TrunkRegistrar struct {
	ua        *sipgo.UserAgent
	logger    *slog.Logger
	ipMatcher *IPAuthMatcher

	mu     sync.RWMutex
	states map[int64]*trunkEntry // keyed by trunk ID
}

const (
	// healthCheckInterval is how often we send OPTIONS pings to trunks.
	healthCheckInterval = 30 * time.Second
	// healthCheckTimeout is the max time to wait for an OPTIONS response.
	healthCheckTimeout = 5 * time.Second
)

// trunkEntry holds per-trunk runtime data.
type trunkEntry struct {
	trunk       models.Trunk
	state       TrunkState
	client      *sipgo.Client
	cancel      context.CancelFunc
	healthClose context.CancelFunc // cancels the health check loop independently
}

// NewTrunkRegistrar creates a trunk registration manager.
func NewTrunkRegistrar(ua *sipgo.UserAgent, logger *slog.Logger) *TrunkRegistrar {
	l := logger.With("subsystem", "trunk-registrar")
	return &TrunkRegistrar{
		ua:        ua,
		logger:    l,
		ipMatcher: NewIPAuthMatcher(l),
		states:    make(map[int64]*trunkEntry),
	}
}

// IPMatcher returns the IP-auth matcher for querying trunk ACLs.
func (tr *TrunkRegistrar) IPMatcher() *IPAuthMatcher {
	return tr.ipMatcher
}

// StartTrunk begins registration for a register-type trunk.
// If the trunk is already running, it is stopped first.
func (tr *TrunkRegistrar) StartTrunk(ctx context.Context, trunk models.Trunk) error {
	if trunk.Type != "register" {
		return fmt.Errorf("trunk %q is type %q, not register", trunk.Name, trunk.Type)
	}
	if !trunk.Enabled {
		tr.setStatus(trunk.ID, trunk.Name, trunk.Type, TrunkStatusDisabled, "")
		return nil
	}

	// Stop existing registration if running.
	tr.StopTrunk(trunk.ID)

	client, err := sipgo.NewClient(tr.ua,
		sipgo.WithClientLogger(tr.logger.With("trunk", trunk.Name)),
	)
	if err != nil {
		return fmt.Errorf("creating sip client for trunk %q: %w", trunk.Name, err)
	}

	// Use a background context for the long-running registration goroutine
	// so it isn't canceled when the calling context (e.g. HTTP request) ends.
	trunkCtx, cancel := context.WithCancel(context.Background())

	entry := &trunkEntry{
		trunk:  trunk,
		client: client,
		cancel: cancel,
		state: TrunkState{
			TrunkID: trunk.ID,
			Name:    trunk.Name,
			Type:    trunk.Type,
			Status:  TrunkStatusRegistering,
		},
	}

	tr.mu.Lock()
	tr.states[trunk.ID] = entry
	tr.mu.Unlock()

	go tr.registrationLoop(trunkCtx, entry)

	// Start a separate health check loop that monitors trunk reachability
	// via OPTIONS pings, independent of the registration cycle.
	healthCtx, healthCancel := context.WithCancel(trunkCtx)
	entry.healthClose = healthCancel
	go tr.healthCheckLoop(healthCtx, entry)

	return nil
}

// StopTrunk cancels the registration loop for a trunk and sends an un-register.
// For IP-auth trunks, this also removes the trunk from the IP matcher.
func (tr *TrunkRegistrar) StopTrunk(trunkID int64) {
	tr.mu.Lock()
	entry, ok := tr.states[trunkID]
	if ok {
		delete(tr.states, trunkID)
	}
	tr.mu.Unlock()

	if !ok {
		return
	}

	// Remove from IP matcher if this was an IP-auth trunk.
	if entry.trunk.Type == "ip" {
		tr.ipMatcher.RemoveTrunk(trunkID)
	}

	// Cancel health check loop first.
	if entry.healthClose != nil {
		entry.healthClose()
	}
	entry.cancel()

	// Best-effort un-register with a short timeout.
	if entry.state.Status == TrunkStatusRegistered && entry.trunk.Type == "register" {
		unregCtx, unregCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer unregCancel()
		if _, err := tr.sendRegister(unregCtx, entry, 0); err != nil {
			tr.logger.Warn("failed to un-register trunk",
				"trunk", entry.trunk.Name,
				"error", err,
			)
		}
	}

	entry.client.Close()
	tr.logger.Info("trunk registration stopped", "trunk", entry.trunk.Name)
}

// StopAllTrunks stops all running trunk registrations and health checks.
// Returns the list of trunk IDs that were stopped.
func (tr *TrunkRegistrar) StopAllTrunks() []int64 {
	tr.mu.Lock()
	ids := make([]int64, 0, len(tr.states))
	for id := range tr.states {
		ids = append(ids, id)
	}
	tr.mu.Unlock()

	for _, id := range ids {
		tr.StopTrunk(id)
	}
	return ids
}

// GetStatus returns the current status for a trunk.
func (tr *TrunkRegistrar) GetStatus(trunkID int64) (TrunkState, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	entry, ok := tr.states[trunkID]
	if !ok {
		return TrunkState{}, false
	}
	return entry.state, true
}

// GetAllStatuses returns a snapshot of all trunk states.
func (tr *TrunkRegistrar) GetAllStatuses() []TrunkState {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	states := make([]TrunkState, 0, len(tr.states))
	for _, entry := range tr.states {
		states = append(states, entry.state)
	}
	return states
}

// registrationLoop runs the registration lifecycle for a single trunk:
// initial register, then periodic re-registration.
func (tr *TrunkRegistrar) registrationLoop(ctx context.Context, entry *trunkEntry) {
	trunk := entry.trunk
	expiry := trunk.RegisterExpiry
	if expiry <= 0 {
		expiry = 300
	}

	tr.logger.Info("starting trunk registration",
		"trunk", trunk.Name,
		"host", trunk.Host,
		"port", trunk.Port,
		"transport", trunk.Transport,
		"expiry", expiry,
	)

	backoff := newBackoff()

	for {
		grantedExpiry, err := tr.sendRegister(ctx, entry, expiry)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			retryDelay := backoff.next()
			tr.logger.Error("trunk registration failed",
				"trunk", trunk.Name,
				"error", err,
				"attempt", backoff.attempt,
				"retry_in", retryDelay.String(),
			)

			now := time.Now()
			tr.mu.Lock()
			if e, ok := tr.states[trunk.ID]; ok {
				e.state.Status = TrunkStatusFailed
				e.state.LastError = err.Error()
				e.state.RetryAttempt = backoff.attempt
				if e.state.FailedAt == nil {
					e.state.FailedAt = &now
				}
			}
			tr.mu.Unlock()

			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
				continue
			}
		}

		// Registration succeeded — use server-granted expiry for timing.
		backoff.reset()
		now := time.Now()
		expiresAt := now.Add(time.Duration(grantedExpiry) * time.Second)
		tr.mu.Lock()
		if e, ok := tr.states[trunk.ID]; ok {
			e.state.Status = TrunkStatusRegistered
			e.state.LastError = ""
			e.state.RetryAttempt = 0
			e.state.FailedAt = nil
			e.state.RegisteredAt = &now
			e.state.ExpiresAt = &expiresAt
		}
		tr.mu.Unlock()

		if grantedExpiry != expiry {
			tr.logger.Info("trunk registered (server adjusted expiry)",
				"trunk", trunk.Name,
				"requested_expiry", expiry,
				"granted_expiry", grantedExpiry,
			)
		} else {
			tr.logger.Info("trunk registered",
				"trunk", trunk.Name,
				"expires_in", grantedExpiry,
			)
		}

		// Re-register before expiry. Use 80% of server-granted expiry as
		// the refresh interval to account for network delays.
		refreshInterval := time.Duration(float64(grantedExpiry)*0.8) * time.Second

		select {
		case <-ctx.Done():
			return
		case <-time.After(refreshInterval):
			tr.logger.Debug("re-registering trunk", "trunk", trunk.Name)
		}
	}
}

// sendRegister sends a SIP REGISTER request with digest auth handling.
// On success it returns the server-granted expiry (from the 200 OK response).
// If the server does not include an expiry, the requested expiry is returned.
func (tr *TrunkRegistrar) sendRegister(ctx context.Context, entry *trunkEntry, expiry int) (int, error) {
	trunk := entry.trunk

	// Build recipient URI (Request-URI for REGISTER).
	recipientStr := fmt.Sprintf("sip:%s:%d", trunk.Host, trunk.Port)
	var recipient sip.Uri
	if err := sip.ParseUri(recipientStr, &recipient); err != nil {
		return 0, fmt.Errorf("parsing recipient uri: %w", err)
	}

	req := sip.NewRequest(sip.REGISTER, recipient)
	req.SetTransport(strings.ToUpper(trunk.Transport))

	// Set From and To headers with the extension's AOR (Address of Record).
	// The registrar uses these to identify which extension is registering.
	username := trunk.Username
	aor := fmt.Sprintf("<sip:%s@%s>", username, trunk.Host)
	req.AppendHeader(sip.NewHeader("From", aor))
	req.AppendHeader(sip.NewHeader("To", aor))

	// Add Contact header with our local address.
	contactURI := fmt.Sprintf("<sip:%s@%s>", username, tr.ua.Hostname())
	req.AppendHeader(sip.NewHeader("Contact", contactURI))

	// Set Expires header.
	req.AppendHeader(sip.NewHeader("Expires", fmt.Sprintf("%d", expiry)))

	// Send initial request.
	tx, err := entry.client.TransactionRequest(ctx, req, sipgo.ClientRequestRegisterBuild)
	if err != nil {
		return 0, fmt.Errorf("sending register: %w", err)
	}

	res, err := getResponse(ctx, tx)
	tx.Terminate()
	if err != nil {
		return 0, fmt.Errorf("waiting for register response: %w", err)
	}

	// Handle 401 Unauthorized — digest authentication.
	if res.StatusCode == 401 || res.StatusCode == 407 {
		authHeader := "WWW-Authenticate"
		authzHeader := "Authorization"
		if res.StatusCode == 407 {
			authHeader = "Proxy-Authenticate"
			authzHeader = "Proxy-Authorization"
		}

		wwwAuth := res.GetHeader(authHeader)
		if wwwAuth == nil {
			return 0, fmt.Errorf("received %d but no %s header", res.StatusCode, authHeader)
		}

		chal, err := digest.ParseChallenge(wwwAuth.Value())
		if err != nil {
			return 0, fmt.Errorf("parsing auth challenge: %w", err)
		}

		// Use auth_username if configured, otherwise username.
		authUser := trunk.Username
		if trunk.AuthUsername != "" {
			authUser = trunk.AuthUsername
		}

		cred, err := digest.Digest(chal, digest.Options{
			Method:   req.Method.String(),
			URI:      recipientStr,
			Username: authUser,
			Password: trunk.Password,
		})
		if err != nil {
			return 0, fmt.Errorf("computing digest: %w", err)
		}

		// Build authenticated request.
		authReq := req.Clone()
		authReq.RemoveHeader("Via")
		authReq.AppendHeader(sip.NewHeader(authzHeader, cred.String()))

		tx2, err := entry.client.TransactionRequest(ctx, authReq,
			sipgo.ClientRequestIncreaseCSEQ,
			sipgo.ClientRequestAddVia,
		)
		if err != nil {
			return 0, fmt.Errorf("sending authenticated register: %w", err)
		}

		res, err = getResponse(ctx, tx2)
		tx2.Terminate()
		if err != nil {
			return 0, fmt.Errorf("waiting for authenticated register response: %w", err)
		}
	}

	if res.StatusCode != 200 {
		return 0, fmt.Errorf("register failed with status %d %s", res.StatusCode, res.Reason)
	}

	// Parse server-granted expiry from the 200 OK response.
	// Per RFC 3261 §10.2.4, the registrar may shorten the requested expiry.
	// Check Contact header expires param first, then Expires header.
	grantedExpiry := expiry
	if contactHdr := res.GetHeader("Contact"); contactHdr != nil {
		if parsed := parseContactExpires(contactHdr.Value()); parsed > 0 {
			grantedExpiry = parsed
		}
	} else if expiresHdr := res.GetHeader("Expires"); expiresHdr != nil {
		if parsed := parseExpiresHeader(expiresHdr.Value()); parsed > 0 {
			grantedExpiry = parsed
		}
	}

	return grantedExpiry, nil
}

// StartHealthCheck begins an OPTIONS ping health check loop for a trunk that
// does not require registration (e.g. IP-auth trunks). For register-type
// trunks, health checking is started automatically by StartTrunk.
//
// For IP-auth trunks, this also registers the trunk's remote_hosts ACL with
// the IP matcher so inbound requests can be matched to this trunk.
func (tr *TrunkRegistrar) StartHealthCheck(ctx context.Context, trunk models.Trunk) error {
	if !trunk.Enabled {
		tr.setStatus(trunk.ID, trunk.Name, trunk.Type, TrunkStatusDisabled, "")
		return nil
	}

	// Stop existing entry if running.
	tr.StopTrunk(trunk.ID)

	// Register IP-auth ACL for inbound matching.
	if trunk.Type == "ip" {
		if err := tr.ipMatcher.AddTrunk(trunk); err != nil {
			return fmt.Errorf("adding ip-auth acl for trunk %q: %w", trunk.Name, err)
		}
	}

	client, err := sipgo.NewClient(tr.ua,
		sipgo.WithClientLogger(tr.logger.With("trunk", trunk.Name)),
	)
	if err != nil {
		return fmt.Errorf("creating sip client for trunk %q: %w", trunk.Name, err)
	}

	// Use a background context so the health check loop isn't canceled
	// when the calling context (e.g. HTTP request) ends.
	trunkCtx, cancel := context.WithCancel(context.Background())
	healthCtx, healthCancel := context.WithCancel(trunkCtx)

	entry := &trunkEntry{
		trunk:       trunk,
		client:      client,
		cancel:      cancel,
		healthClose: healthCancel,
		state: TrunkState{
			TrunkID: trunk.ID,
			Name:    trunk.Name,
			Type:    trunk.Type,
			Status:  TrunkStatusUnregistered,
		},
	}

	tr.mu.Lock()
	tr.states[trunk.ID] = entry
	tr.mu.Unlock()

	go tr.healthCheckLoop(healthCtx, entry)
	return nil
}

// healthCheckLoop periodically sends OPTIONS pings to a trunk and updates
// the OptionsHealthy flag. If the trunk was previously registered and the
// OPTIONS ping fails, the status transitions to failed.
func (tr *TrunkRegistrar) healthCheckLoop(ctx context.Context, entry *trunkEntry) {
	trunk := entry.trunk

	tr.logger.Info("starting health check loop",
		"trunk", trunk.Name,
		"interval", healthCheckInterval.String(),
	)

	// Wait one interval before the first check to allow registration to complete.
	select {
	case <-ctx.Done():
		return
	case <-time.After(healthCheckInterval):
	}

	for {
		err := tr.sendOptionsEntry(ctx, entry)

		tr.mu.Lock()
		if e, ok := tr.states[trunk.ID]; ok {
			now := time.Now()
			if err == nil {
				e.state.OptionsHealthy = true
				e.state.LastOptionsAt = &now

				// For IP-auth trunks, OPTIONS success means the trunk is reachable.
				if e.state.Type == "ip" && e.state.Status != TrunkStatusRegistered {
					e.state.Status = TrunkStatusRegistered
					e.state.FailedAt = nil
					e.state.LastError = ""
				}
			} else if ctx.Err() == nil {
				e.state.OptionsHealthy = false

				// For IP-auth trunks, OPTIONS failure means the trunk is unreachable.
				if e.state.Type == "ip" {
					e.state.Status = TrunkStatusFailed
					e.state.LastError = err.Error()
					if e.state.FailedAt == nil {
						e.state.FailedAt = &now
					}
				}

				tr.logger.Warn("health check failed",
					"trunk", trunk.Name,
					"error", err,
				)
			}
		}
		tr.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-time.After(healthCheckInterval):
		}
	}
}

// sendOptionsEntry sends a SIP OPTIONS request using an existing trunk entry's
// client. This avoids creating a new client for each health check ping.
func (tr *TrunkRegistrar) sendOptionsEntry(ctx context.Context, entry *trunkEntry) error {
	trunk := entry.trunk

	recipientStr := fmt.Sprintf("sip:%s:%d", trunk.Host, trunk.Port)
	var recipient sip.Uri
	if err := sip.ParseUri(recipientStr, &recipient); err != nil {
		return fmt.Errorf("parsing recipient uri: %w", err)
	}

	req := sip.NewRequest(sip.OPTIONS, recipient)
	req.SetTransport(strings.ToUpper(trunk.Transport))

	pingCtx, pingCancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer pingCancel()

	tx, err := entry.client.TransactionRequest(pingCtx, req, sipgo.ClientRequestBuild)
	if err != nil {
		return fmt.Errorf("sending options: %w", err)
	}

	res, err := getResponse(pingCtx, tx)
	tx.Terminate()
	if err != nil {
		return fmt.Errorf("waiting for options response: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("options ping returned status %d %s", res.StatusCode, res.Reason)
	}

	return nil
}

// TestRegister sends a one-shot SIP REGISTER request to verify that a trunk's
// credentials are valid and the registrar is reachable. Unlike StartTrunk, this
// does not start a registration loop — it returns after a single attempt.
func (tr *TrunkRegistrar) TestRegister(ctx context.Context, trunk models.Trunk) error {
	client, err := sipgo.NewClient(tr.ua,
		sipgo.WithClientLogger(tr.logger.With("trunk", trunk.Name)),
	)
	if err != nil {
		return fmt.Errorf("creating sip client: %w", err)
	}
	defer client.Close()

	entry := &trunkEntry{
		trunk:  trunk,
		client: client,
		state: TrunkState{
			TrunkID: trunk.ID,
			Name:    trunk.Name,
			Type:    trunk.Type,
			Status:  TrunkStatusRegistering,
		},
	}

	expiry := trunk.RegisterExpiry
	if expiry <= 0 {
		expiry = 300
	}

	_, err = tr.sendRegister(ctx, entry, expiry)
	return err
}

// SendOptions sends a SIP OPTIONS ping to a trunk and returns an error if
// the trunk is unreachable or responds with a non-2xx status.
func (tr *TrunkRegistrar) SendOptions(ctx context.Context, trunk models.Trunk) error {
	client, err := sipgo.NewClient(tr.ua,
		sipgo.WithClientLogger(tr.logger.With("trunk", trunk.Name)),
	)
	if err != nil {
		return fmt.Errorf("creating sip client: %w", err)
	}
	defer client.Close()

	recipientStr := fmt.Sprintf("sip:%s:%d", trunk.Host, trunk.Port)
	var recipient sip.Uri
	if err := sip.ParseUri(recipientStr, &recipient); err != nil {
		return fmt.Errorf("parsing recipient uri: %w", err)
	}

	req := sip.NewRequest(sip.OPTIONS, recipient)
	req.SetTransport(strings.ToUpper(trunk.Transport))

	tx, err := client.TransactionRequest(ctx, req, sipgo.ClientRequestBuild)
	if err != nil {
		return fmt.Errorf("sending options: %w", err)
	}

	res, err := getResponse(ctx, tx)
	tx.Terminate()
	if err != nil {
		return fmt.Errorf("waiting for options response: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("options ping returned status %d %s", res.StatusCode, res.Reason)
	}

	return nil
}

// setStatus updates the status of a trunk in the state map.
func (tr *TrunkRegistrar) setStatus(trunkID int64, name, trunkType string, status TrunkStatus, lastErr string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	entry, ok := tr.states[trunkID]
	if ok {
		entry.state.Status = status
		entry.state.LastError = lastErr
	} else {
		tr.states[trunkID] = &trunkEntry{
			state: TrunkState{
				TrunkID:   trunkID,
				Name:      name,
				Type:      trunkType,
				Status:    status,
				LastError: lastErr,
			},
		}
	}
}

// getResponse waits for the first response from a SIP client transaction.
func getResponse(ctx context.Context, tx sip.ClientTransaction) (*sip.Response, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-tx.Done():
		return nil, fmt.Errorf("transaction terminated: %w", tx.Err())
	case res := <-tx.Responses():
		return res, nil
	}
}

// parseContactExpires extracts the expires parameter from a Contact header value.
// Contact headers may contain: <sip:user@host>;expires=3600
// Returns 0 if no expires parameter is found or parsing fails.
func parseContactExpires(contactValue string) int {
	// Look for ;expires= parameter (case-insensitive).
	lower := strings.ToLower(contactValue)
	idx := strings.Index(lower, ";expires=")
	if idx < 0 {
		return 0
	}
	rest := contactValue[idx+len(";expires="):]

	// The value ends at the next semicolon, comma, or end of string.
	end := strings.IndexAny(rest, ";,> \t")
	if end > 0 {
		rest = rest[:end]
	}

	val, err := strconv.Atoi(strings.TrimSpace(rest))
	if err != nil {
		return 0
	}
	return val
}

// parseExpiresHeader parses an Expires header value (a plain integer of seconds).
// Returns 0 if parsing fails.
func parseExpiresHeader(value string) int {
	val, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return val
}

// backoff implements exponential backoff with jitter for registration retries.
// Jitter prevents thundering herd when multiple trunks fail simultaneously.
type backoff struct {
	attempt   int
	baseDelay time.Duration
	maxDelay  time.Duration
}

func newBackoff() *backoff {
	return &backoff{
		baseDelay: 5 * time.Second,
		maxDelay:  5 * time.Minute,
	}
}

func (b *backoff) next() time.Duration {
	d := b.current()
	b.attempt++
	return d
}

func (b *backoff) current() time.Duration {
	d := b.baseDelay
	for i := 0; i < b.attempt; i++ {
		d *= 2
		if d > b.maxDelay {
			d = b.maxDelay
			break
		}
	}
	// Add ±20% jitter to prevent thundering herd.
	jitter := float64(d) * 0.2 * (2*rand.Float64() - 1)
	d += time.Duration(jitter)
	if d < 0 {
		d = b.baseDelay
	}
	return d
}

func (b *backoff) reset() {
	b.attempt = 0
}
