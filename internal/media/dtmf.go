package media

import (
	"errors"
	"log/slog"
	"net"
	"strconv"
	"strings"
)

const (
	// PayloadTelephoneEvent is the standard dynamic RTP payload type for
	// RFC 2833 telephone-event (DTMF). Commonly negotiated as 101.
	PayloadTelephoneEvent = 101
)

// DTMFEvent represents an RFC 2833 telephone-event payload.
// The payload format (RFC 4733 §2.3) is:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     event     |E|R| volume    |          duration             |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
type DTMFEvent struct {
	Event    uint8  // DTMF digit: 0-9 = digits, 10 = *, 11 = #, 12-15 = A-D
	End      bool   // E bit: marks end of event
	Volume   uint8  // power level in dBm0 (0-63)
	Duration uint16 // event duration in timestamp units
}

// dtmfPayloadSize is the size of an RFC 2833 telephone-event payload.
const dtmfPayloadSize = 4

// ParseDTMFEvent parses an RFC 2833 telephone-event payload from raw bytes.
// Returns nil if the payload is too short.
func ParseDTMFEvent(payload []byte) *DTMFEvent {
	if len(payload) < dtmfPayloadSize {
		return nil
	}
	return &DTMFEvent{
		Event:    payload[0],
		End:      payload[1]&0x80 != 0,
		Volume:   payload[1] & 0x3F,
		Duration: uint16(payload[2])<<8 | uint16(payload[3]),
	}
}

// DTMFEventName returns the human-readable name of a DTMF event code.
func DTMFEventName(event uint8) string {
	switch {
	case event <= 9:
		return string(rune('0' + event))
	case event == 10:
		return "*"
	case event == 11:
		return "#"
	case event >= 12 && event <= 15:
		return string(rune('A' + event - 12))
	default:
		return "?"
	}
}

// SIP INFO DTMF fallback
//
// Some endpoints send DTMF digits via SIP INFO instead of RFC 2833 in-band
// telephone-event. Two body formats are common:
//
//  1. Content-Type: application/dtmf-relay
//     Signal=5\r\nDuration=160\r\n
//
//  2. Content-Type: application/dtmf
//     5

// DTMFInfo represents a DTMF digit received via SIP INFO request.
type DTMFInfo struct {
	Signal   string // The DTMF digit: "0"-"9", "*", "#", "A"-"D"
	Duration int    // Duration in milliseconds (0 if not specified)
}

// ErrInvalidDTMFInfo is returned when a SIP INFO body cannot be parsed as DTMF.
var ErrInvalidDTMFInfo = errors.New("invalid dtmf info body")

// validDTMFSignals is the set of valid DTMF signal characters.
var validDTMFSignals = map[string]bool{
	"0": true, "1": true, "2": true, "3": true, "4": true,
	"5": true, "6": true, "7": true, "8": true, "9": true,
	"*": true, "#": true,
	"A": true, "B": true, "C": true, "D": true,
}

// ParseDTMFInfoRelay parses a SIP INFO body with Content-Type application/dtmf-relay.
// The expected format is:
//
//	Signal=<digit>\r\nDuration=<ms>\r\n
//
// Signal is required. Duration defaults to 0 if missing or unparseable.
func ParseDTMFInfoRelay(body []byte) (*DTMFInfo, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return nil, ErrInvalidDTMFInfo
	}

	info := &DTMFInfo{}
	foundSignal := false

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch strings.ToLower(key) {
		case "signal":
			sig := strings.ToUpper(value)
			if !validDTMFSignals[sig] {
				return nil, ErrInvalidDTMFInfo
			}
			info.Signal = sig
			foundSignal = true
		case "duration":
			d, err := strconv.Atoi(value)
			if err == nil && d >= 0 {
				info.Duration = d
			}
		}
	}

	if !foundSignal {
		return nil, ErrInvalidDTMFInfo
	}
	return info, nil
}

// ParseDTMFInfoBody parses a SIP INFO body with Content-Type application/dtmf.
// The body should contain a single DTMF digit character.
func ParseDTMFInfoBody(body []byte) (*DTMFInfo, error) {
	sig := strings.TrimSpace(string(body))
	sig = strings.ToUpper(sig)
	if !validDTMFSignals[sig] {
		return nil, ErrInvalidDTMFInfo
	}
	return &DTMFInfo{Signal: sig}, nil
}

// ParseSIPInfoDTMF detects and parses DTMF from a SIP INFO request body based
// on the Content-Type header. Supported content types:
//   - application/dtmf-relay
//   - application/dtmf
//
// Returns ErrInvalidDTMFInfo if the content type is unsupported or the body
// cannot be parsed.
func ParseSIPInfoDTMF(contentType string, body []byte) (*DTMFInfo, error) {
	ct := strings.TrimSpace(strings.ToLower(contentType))
	// Strip any parameters (e.g., charset).
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}

	switch ct {
	case "application/dtmf-relay":
		return ParseDTMFInfoRelay(body)
	case "application/dtmf":
		return ParseDTMFInfoBody(body)
	default:
		return nil, ErrInvalidDTMFInfo
	}
}

// StartDTMFRelay creates and starts a relay that passes through RFC 2833
// telephone-event (DTMF) packets between the two legs of a session. The
// relay filters for the configured telephone-event payload type (commonly 101).
//
// This uses the same Relay infrastructure as codec relays — telephone-event
// is just another RTP payload type that gets forwarded transparently.
func StartDTMFRelay(session *Session, callerRemote, calleeRemote *net.UDPAddr, logger *slog.Logger) *Relay {
	relay := NewRelay(session, callerRemote, calleeRemote, []int{PayloadTelephoneEvent}, logger)
	relay.Start()
	return relay
}

// StartAudioWithDTMFRelay creates and starts a relay that passes through both
// the specified audio codec and RFC 2833 telephone-event packets. This is the
// common case: an audio stream carrying voice plus in-band DTMF signaling.
func StartAudioWithDTMFRelay(session *Session, callerRemote, calleeRemote *net.UDPAddr, audioPayloadType int, logger *slog.Logger) *Relay {
	relay := NewRelay(session, callerRemote, calleeRemote, []int{audioPayloadType, PayloadTelephoneEvent}, logger)
	relay.Start()
	return relay
}
