package media

import (
	"log/slog"
	"net"
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
