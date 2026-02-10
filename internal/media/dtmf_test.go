package media

import (
	"log/slog"
	"net"
	"testing"
	"time"
)

func TestParseDTMFEvent(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		expected *DTMFEvent
	}{
		{
			"digit 1 start",
			[]byte{0x01, 0x0A, 0x00, 0xA0},
			&DTMFEvent{Event: 1, End: false, Volume: 10, Duration: 160},
		},
		{
			"digit 1 end",
			[]byte{0x01, 0x8A, 0x03, 0x20},
			&DTMFEvent{Event: 1, End: true, Volume: 10, Duration: 800},
		},
		{
			"digit 0",
			[]byte{0x00, 0x0A, 0x00, 0xA0},
			&DTMFEvent{Event: 0, End: false, Volume: 10, Duration: 160},
		},
		{
			"star",
			[]byte{0x0A, 0x0A, 0x00, 0xA0},
			&DTMFEvent{Event: 10, End: false, Volume: 10, Duration: 160},
		},
		{
			"hash",
			[]byte{0x0B, 0x0A, 0x00, 0xA0},
			&DTMFEvent{Event: 11, End: false, Volume: 10, Duration: 160},
		},
		{
			"too short",
			[]byte{0x01, 0x0A, 0x00},
			nil,
		},
		{
			"empty",
			nil,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDTMFEvent(tt.payload)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil DTMFEvent, got nil")
			}
			if got.Event != tt.expected.Event {
				t.Errorf("Event = %d, want %d", got.Event, tt.expected.Event)
			}
			if got.End != tt.expected.End {
				t.Errorf("End = %v, want %v", got.End, tt.expected.End)
			}
			if got.Volume != tt.expected.Volume {
				t.Errorf("Volume = %d, want %d", got.Volume, tt.expected.Volume)
			}
			if got.Duration != tt.expected.Duration {
				t.Errorf("Duration = %d, want %d", got.Duration, tt.expected.Duration)
			}
		})
	}
}

func TestDTMFEventName(t *testing.T) {
	tests := []struct {
		event    uint8
		expected string
	}{
		{0, "0"}, {1, "1"}, {2, "2"}, {3, "3"}, {4, "4"},
		{5, "5"}, {6, "6"}, {7, "7"}, {8, "8"}, {9, "9"},
		{10, "*"}, {11, "#"},
		{12, "A"}, {13, "B"}, {14, "C"}, {15, "D"},
		{16, "?"}, {255, "?"},
	}
	for _, tt := range tests {
		got := DTMFEventName(tt.event)
		if got != tt.expected {
			t.Errorf("DTMFEventName(%d) = %q, want %q", tt.event, got, tt.expected)
		}
	}
}

func TestDTMFRelay(t *testing.T) {
	logger := slog.Default()

	callerPair, callerLocalAddr := allocateTestPair(t)
	defer callerPair.Close()
	calleePair, calleeLocalAddr := allocateTestPair(t)
	defer calleePair.Close()

	callerPhone, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen caller phone: %v", err)
	}
	defer callerPhone.Close()

	calleePhone, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen callee phone: %v", err)
	}
	defer calleePhone.Close()

	callerPhoneAddr := callerPhone.LocalAddr().(*net.UDPAddr)
	calleePhoneAddr := calleePhone.LocalAddr().(*net.UDPAddr)

	session := &Session{
		ID:        "test-session-dtmf",
		CallID:    "test-call-dtmf",
		CallerLeg: callerPair,
		CalleeLeg: calleePair,
		CreatedAt: time.Now(),
		state:     SessionStateNew,
	}

	relay := StartDTMFRelay(session, callerPhoneAddr, calleePhoneAddr, logger)
	defer relay.Stop()

	if session.State() != SessionStateActive {
		t.Fatalf("expected session state Active, got %s", session.State())
	}

	t.Run("caller to callee DTMF forwarded", func(t *testing.T) {
		// RFC 2833 telephone-event: digit 1, volume 10, duration 160
		dtmfPayload := []byte{0x01, 0x0A, 0x00, 0xA0}
		pkt := makeTestRTPPacket(PayloadTelephoneEvent, dtmfPayload)

		_, err := callerPhone.WriteToUDP(pkt, callerLocalAddr)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		calleePhone.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, maxRTPPacket)
		n, _, err := calleePhone.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("callee phone read: %v", err)
		}

		if n != len(pkt) {
			t.Errorf("received %d bytes, want %d", n, len(pkt))
		}
		if rtpPayloadType(buf[:n]) != PayloadTelephoneEvent {
			t.Errorf("payload type = %d, want %d", rtpPayloadType(buf[:n]), PayloadTelephoneEvent)
		}

		// Verify the DTMF payload is preserved.
		event := ParseDTMFEvent(buf[minRTPHeader:n])
		if event == nil {
			t.Fatal("failed to parse forwarded DTMF event")
		}
		if event.Event != 1 {
			t.Errorf("DTMF event = %d, want 1", event.Event)
		}
	})

	t.Run("callee to caller DTMF forwarded", func(t *testing.T) {
		// RFC 2833 telephone-event: digit 5 end, volume 10, duration 800
		dtmfPayload := []byte{0x05, 0x8A, 0x03, 0x20}
		pkt := makeTestRTPPacket(PayloadTelephoneEvent, dtmfPayload)

		_, err := calleePhone.WriteToUDP(pkt, calleeLocalAddr)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		callerPhone.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, maxRTPPacket)
		n, _, err := callerPhone.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("caller phone read: %v", err)
		}

		if n != len(pkt) {
			t.Errorf("received %d bytes, want %d", n, len(pkt))
		}

		event := ParseDTMFEvent(buf[minRTPHeader:n])
		if event == nil {
			t.Fatal("failed to parse forwarded DTMF event")
		}
		if event.Event != 5 {
			t.Errorf("DTMF event = %d, want 5", event.Event)
		}
		if !event.End {
			t.Error("expected End bit set")
		}
	})

	t.Run("non-DTMF packet dropped", func(t *testing.T) {
		// Send a PCMA packet — should be dropped by the DTMF-only relay.
		pkt := makeTestRTPPacket(PayloadPCMA, []byte{0x01, 0x02})

		_, err := callerPhone.WriteToUDP(pkt, callerLocalAddr)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		calleePhone.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		buf := make([]byte, maxRTPPacket)
		_, _, err = calleePhone.ReadFromUDP(buf)
		if err == nil {
			t.Error("expected timeout (packet should be dropped), but received data")
		}
	})
}

func TestAudioWithDTMFRelay(t *testing.T) {
	logger := slog.Default()

	callerPair, callerLocalAddr := allocateTestPair(t)
	defer callerPair.Close()
	calleePair, calleeLocalAddr := allocateTestPair(t)
	defer calleePair.Close()

	callerPhone, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen caller phone: %v", err)
	}
	defer callerPhone.Close()

	calleePhone, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen callee phone: %v", err)
	}
	defer calleePhone.Close()

	callerPhoneAddr := callerPhone.LocalAddr().(*net.UDPAddr)
	calleePhoneAddr := calleePhone.LocalAddr().(*net.UDPAddr)

	session := &Session{
		ID:        "test-session-audio-dtmf",
		CallID:    "test-call-audio-dtmf",
		CallerLeg: callerPair,
		CalleeLeg: calleePair,
		CreatedAt: time.Now(),
		state:     SessionStateNew,
	}

	// Start combined PCMA + DTMF relay.
	relay := StartAudioWithDTMFRelay(session, callerPhoneAddr, calleePhoneAddr, PayloadPCMA, logger)
	defer relay.Stop()

	t.Run("PCMA forwarded caller to callee", func(t *testing.T) {
		pkt := makeTestRTPPacket(PayloadPCMA, []byte{0xD5, 0xD5, 0xD5, 0xD5})

		_, err := callerPhone.WriteToUDP(pkt, callerLocalAddr)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		calleePhone.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, maxRTPPacket)
		n, _, err := calleePhone.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("callee phone read: %v", err)
		}

		if rtpPayloadType(buf[:n]) != PayloadPCMA {
			t.Errorf("payload type = %d, want %d", rtpPayloadType(buf[:n]), PayloadPCMA)
		}
	})

	t.Run("DTMF forwarded caller to callee", func(t *testing.T) {
		dtmfPayload := []byte{0x09, 0x0A, 0x00, 0xA0} // digit 9
		pkt := makeTestRTPPacket(PayloadTelephoneEvent, dtmfPayload)

		_, err := callerPhone.WriteToUDP(pkt, callerLocalAddr)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		calleePhone.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, maxRTPPacket)
		n, _, err := calleePhone.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("callee phone read: %v", err)
		}

		if rtpPayloadType(buf[:n]) != PayloadTelephoneEvent {
			t.Errorf("payload type = %d, want %d", rtpPayloadType(buf[:n]), PayloadTelephoneEvent)
		}
	})

	t.Run("DTMF forwarded callee to caller", func(t *testing.T) {
		dtmfPayload := []byte{0x0B, 0x8A, 0x03, 0x20} // hash end
		pkt := makeTestRTPPacket(PayloadTelephoneEvent, dtmfPayload)

		_, err := calleePhone.WriteToUDP(pkt, calleeLocalAddr)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		callerPhone.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, maxRTPPacket)
		n, _, err := callerPhone.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("caller phone read: %v", err)
		}

		if rtpPayloadType(buf[:n]) != PayloadTelephoneEvent {
			t.Errorf("payload type = %d, want %d", rtpPayloadType(buf[:n]), PayloadTelephoneEvent)
		}
	})

	t.Run("PCMU dropped", func(t *testing.T) {
		// PCMU should be dropped — only PCMA + telephone-event are allowed.
		pkt := makeTestRTPPacket(PayloadPCMU, []byte{0x01, 0x02})

		_, err := callerPhone.WriteToUDP(pkt, callerLocalAddr)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		calleePhone.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		buf := make([]byte, maxRTPPacket)
		_, _, err = calleePhone.ReadFromUDP(buf)
		if err == nil {
			t.Error("expected timeout (packet should be dropped), but received data")
		}
	})
}
