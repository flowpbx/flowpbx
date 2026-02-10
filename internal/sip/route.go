package sip

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/flowpbx/flowpbx/internal/database"
	"github.com/flowpbx/flowpbx/internal/database/models"
)

// RouteResult describes where a call should be sent.
type RouteResult struct {
	// TargetExtension is the extension being called.
	TargetExtension *models.Extension

	// Contacts are the active registrations to ring (may be multiple for
	// multi-device support). Only includes non-expired registrations.
	Contacts []models.Registration
}

// CallRouter resolves call targets and returns the information needed to
// deliver the call (registered contacts, DND status, etc.).
type CallRouter struct {
	extensions    database.ExtensionRepository
	registrations database.RegistrationRepository
	logger        *slog.Logger
}

// NewCallRouter creates a new CallRouter.
func NewCallRouter(
	extensions database.ExtensionRepository,
	registrations database.RegistrationRepository,
	logger *slog.Logger,
) *CallRouter {
	return &CallRouter{
		extensions:    extensions,
		registrations: registrations,
		logger:        logger.With("subsystem", "router"),
	}
}

// RouteInternalCall resolves an internal (extension-to-extension) call.
// It looks up the target extension and finds all active registrations.
//
// Returns an error with a SIP-appropriate status code:
//   - ErrDND (486): target extension has Do Not Disturb enabled
//   - ErrNoRegistrations (480): target has no active registrations
//   - ErrExtensionNotFound (404): target extension does not exist
func (r *CallRouter) RouteInternalCall(ctx context.Context, ic *InviteContext) (*RouteResult, error) {
	if ic.TargetExtension == nil {
		return nil, ErrExtensionNotFound
	}

	ext := ic.TargetExtension

	r.logger.Debug("routing internal call",
		"caller", ic.CallerIDNum,
		"target", ext.Extension,
		"target_id", ext.ID,
	)

	// Check if the target has DND enabled.
	if ext.DND {
		r.logger.Info("target extension has dnd enabled",
			"extension", ext.Extension,
		)
		return nil, ErrDND
	}

	// Look up all active registrations for the target extension.
	regs, err := r.registrations.GetByExtensionID(ctx, ext.ID)
	if err != nil {
		return nil, fmt.Errorf("looking up registrations for extension %s: %w", ext.Extension, err)
	}

	// Filter out expired registrations (belt-and-suspenders; the DB cleanup
	// runs periodically but there can be a small window).
	now := time.Now()
	active := make([]models.Registration, 0, len(regs))
	for _, reg := range regs {
		if reg.Expires.After(now) {
			active = append(active, reg)
		}
	}

	if len(active) == 0 {
		r.logger.Info("no active registrations for target extension",
			"extension", ext.Extension,
		)
		return nil, ErrNoRegistrations
	}

	r.logger.Info("internal call routed",
		"caller", ic.CallerIDNum,
		"target", ext.Extension,
		"contacts", len(active),
	)

	return &RouteResult{
		TargetExtension: ext,
		Contacts:        active,
	}, nil
}

// Routing errors with SIP-semantic meaning. Callers should map these to the
// appropriate SIP response code.
var (
	ErrExtensionNotFound = fmt.Errorf("extension not found")
	ErrDND               = fmt.Errorf("do not disturb enabled")
	ErrNoRegistrations   = fmt.Errorf("no active registrations")
)
