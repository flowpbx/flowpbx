package sip

import (
	"bytes"
	"log/slog"
	"strings"
	"sync/atomic"
)

// SIPLogVerbosity controls how much of each SIP message is logged.
type SIPLogVerbosity int32

const (
	// SIPLogOff disables SIP message tracing.
	SIPLogOff SIPLogVerbosity = iota
	// SIPLogHeaders logs only the start line and headers (no SDP body).
	SIPLogHeaders
	// SIPLogFull logs the complete raw SIP message including SDP body.
	SIPLogFull
)

// ParseSIPLogVerbosity converts a string setting to a SIPLogVerbosity value.
func ParseSIPLogVerbosity(s string) SIPLogVerbosity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "headers":
		return SIPLogHeaders
	case "full":
		return SIPLogFull
	default:
		return SIPLogOff
	}
}

// String returns the string representation of the verbosity level.
func (v SIPLogVerbosity) String() string {
	switch v {
	case SIPLogHeaders:
		return "headers"
	case SIPLogFull:
		return "full"
	default:
		return "off"
	}
}

// MessageTracer implements the sipgo sip.SIPTracer interface for structured
// logging of raw SIP messages at configurable verbosity levels.
type MessageTracer struct {
	logger    *slog.Logger
	verbosity atomic.Int32
}

// NewMessageTracer creates a new SIP message tracer.
func NewMessageTracer(logger *slog.Logger, verbosity SIPLogVerbosity) *MessageTracer {
	t := &MessageTracer{
		logger: logger.With("subsystem", "tracer"),
	}
	t.verbosity.Store(int32(verbosity))
	return t
}

// SetVerbosity updates the tracing verbosity level at runtime.
func (t *MessageTracer) SetVerbosity(v SIPLogVerbosity) {
	t.verbosity.Store(int32(v))
	t.logger.Info("sip message tracing verbosity changed", "verbosity", v.String())
}

// Verbosity returns the current tracing verbosity level.
func (t *MessageTracer) Verbosity() SIPLogVerbosity {
	return SIPLogVerbosity(t.verbosity.Load())
}

// SIPTraceRead is called by sipgo when raw SIP bytes are read from the network.
func (t *MessageTracer) SIPTraceRead(transport string, laddr string, raddr string, sipmsg []byte) {
	v := t.Verbosity()
	if v == SIPLogOff {
		return
	}

	msg := t.formatMessage(sipmsg, v)
	t.logger.Debug("sip recv",
		"direction", "recv",
		"transport", transport,
		"local_addr", laddr,
		"remote_addr", raddr,
		"message", msg,
	)
}

// SIPTraceWrite is called by sipgo when raw SIP bytes are written to the network.
func (t *MessageTracer) SIPTraceWrite(transport string, laddr string, raddr string, sipmsg []byte) {
	v := t.Verbosity()
	if v == SIPLogOff {
		return
	}

	msg := t.formatMessage(sipmsg, v)
	t.logger.Debug("sip send",
		"direction", "send",
		"transport", transport,
		"local_addr", laddr,
		"remote_addr", raddr,
		"message", msg,
	)
}

// formatMessage applies the verbosity filter to the raw SIP message bytes.
func (t *MessageTracer) formatMessage(sipmsg []byte, v SIPLogVerbosity) string {
	if v == SIPLogFull {
		return string(sipmsg)
	}

	// SIPLogHeaders: strip the body (everything after the blank line \r\n\r\n).
	idx := bytes.Index(sipmsg, []byte("\r\n\r\n"))
	if idx >= 0 {
		return string(sipmsg[:idx])
	}
	// No blank line found — return the whole message (likely malformed or header-only).
	return string(sipmsg)
}
