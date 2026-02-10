package media

import (
	"log/slog"
	"net"
	"testing"
	"time"
)

// makeTestRTPPacket creates a minimal RTP packet with the given payload type and payload.
func makeTestRTPPacket(payloadType int, payload []byte) []byte {
	// Minimal 12-byte RTP header:
	// Byte 0: V=2, P=0, X=0, CC=0 → 0x80
	// Byte 1: M=0, PT=payloadType
	// Bytes 2-3: sequence number
	// Bytes 4-7: timestamp
	// Bytes 8-11: SSRC
	header := []byte{
		0x80, byte(payloadType & 0x7F),
		0x00, 0x01, // seq=1
		0x00, 0x00, 0x00, 0xA0, // timestamp
		0x00, 0x00, 0x00, 0x01, // SSRC
	}
	return append(header, payload...)
}

func TestRtpPayloadType(t *testing.T) {
	tests := []struct {
		name     string
		packet   []byte
		expected int
	}{
		{"PCMU", makeTestRTPPacket(PayloadPCMU, []byte{0xFF}), PayloadPCMU},
		{"PCMA", makeTestRTPPacket(PayloadPCMA, []byte{0xFF}), PayloadPCMA},
		{"Opus", makeTestRTPPacket(PayloadOpus, []byte{0xFF}), PayloadOpus},
		{"with marker bit", append([]byte{0x80, 0x80 | byte(PayloadPCMA)}, make([]byte, 10)...), PayloadPCMA},
		{"too small", []byte{0x80, 0x08}, -1},
		{"empty", nil, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rtpPayloadType(tt.packet)
			if got != tt.expected {
				t.Errorf("rtpPayloadType() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// allocateTestPair creates a bound UDP socket pair on localhost for testing.
func allocateTestPair(t *testing.T) (*SocketPair, *net.UDPAddr) {
	t.Helper()

	rtpAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	rtpConn, err := net.ListenUDP("udp", rtpAddr)
	if err != nil {
		t.Fatalf("listen rtp: %v", err)
	}

	rtcpAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	rtcpConn, err := net.ListenUDP("udp", rtcpAddr)
	if err != nil {
		rtpConn.Close()
		t.Fatalf("listen rtcp: %v", err)
	}

	localAddr := rtpConn.LocalAddr().(*net.UDPAddr)
	pair := &SocketPair{
		Ports:    PortPair{RTP: localAddr.Port, RTCP: rtcpConn.LocalAddr().(*net.UDPAddr).Port},
		RTPConn:  rtpConn,
		RTCPConn: rtcpConn,
	}
	return pair, localAddr
}

func TestPCMARelay(t *testing.T) {
	logger := slog.Default()

	// Create caller and callee socket pairs (these are the proxy's local sockets).
	callerPair, callerLocalAddr := allocateTestPair(t)
	defer callerPair.Close()
	calleePair, calleeLocalAddr := allocateTestPair(t)
	defer calleePair.Close()

	// Create "remote endpoint" sockets simulating the actual caller and callee phones.
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
		ID:        "test-session-1",
		CallID:    "test-call-1",
		CallerLeg: callerPair,
		CalleeLeg: calleePair,
		CreatedAt: time.Now(),
		state:     SessionStateNew,
	}

	// Start PCMA relay:
	// CallerLeg receives from callerPhone, forwards via CalleeLeg to calleePhone.
	// CalleeLeg receives from calleePhone, forwards via CallerLeg to callerPhone.
	relay := StartPCMARelay(session, callerPhoneAddr, calleePhoneAddr, logger)
	defer relay.Stop()

	if session.State() != SessionStateActive {
		t.Fatalf("expected session state Active, got %s", session.State())
	}

	t.Run("caller to callee PCMA forwarded", func(t *testing.T) {
		pkt := makeTestRTPPacket(PayloadPCMA, []byte{0xD5, 0xD5, 0xD5, 0xD5})

		// Caller phone sends to caller leg's local address (the proxy).
		_, err := callerPhone.WriteToUDP(pkt, callerLocalAddr)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		// Callee phone should receive the forwarded packet.
		calleePhone.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, maxRTPPacket)
		n, _, err := calleePhone.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("callee phone read: %v", err)
		}

		if n != len(pkt) {
			t.Errorf("received %d bytes, want %d", n, len(pkt))
		}
		if rtpPayloadType(buf[:n]) != PayloadPCMA {
			t.Errorf("payload type = %d, want %d", rtpPayloadType(buf[:n]), PayloadPCMA)
		}
	})

	t.Run("callee to caller PCMA forwarded", func(t *testing.T) {
		pkt := makeTestRTPPacket(PayloadPCMA, []byte{0xAA, 0xBB, 0xCC})

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
		if rtpPayloadType(buf[:n]) != PayloadPCMA {
			t.Errorf("payload type = %d, want %d", rtpPayloadType(buf[:n]), PayloadPCMA)
		}
	})

	t.Run("non-PCMA packet dropped", func(t *testing.T) {
		// Send a PCMU packet — should be dropped by the PCMA-only relay.
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

	t.Run("too-small packet dropped", func(t *testing.T) {
		// Send a tiny packet — should be dropped.
		_, err := callerPhone.WriteToUDP([]byte{0x80, 0x08}, callerLocalAddr)
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

func TestPCMURelay(t *testing.T) {
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
		ID:        "test-session-pcmu",
		CallID:    "test-call-pcmu",
		CallerLeg: callerPair,
		CalleeLeg: calleePair,
		CreatedAt: time.Now(),
		state:     SessionStateNew,
	}

	relay := StartPCMURelay(session, callerPhoneAddr, calleePhoneAddr, logger)
	defer relay.Stop()

	if session.State() != SessionStateActive {
		t.Fatalf("expected session state Active, got %s", session.State())
	}

	t.Run("caller to callee PCMU forwarded", func(t *testing.T) {
		pkt := makeTestRTPPacket(PayloadPCMU, []byte{0xFE, 0xFE, 0xFE, 0xFE})

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
		if rtpPayloadType(buf[:n]) != PayloadPCMU {
			t.Errorf("payload type = %d, want %d", rtpPayloadType(buf[:n]), PayloadPCMU)
		}
	})

	t.Run("callee to caller PCMU forwarded", func(t *testing.T) {
		pkt := makeTestRTPPacket(PayloadPCMU, []byte{0x11, 0x22, 0x33})

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
		if rtpPayloadType(buf[:n]) != PayloadPCMU {
			t.Errorf("payload type = %d, want %d", rtpPayloadType(buf[:n]), PayloadPCMU)
		}
	})

	t.Run("non-PCMU packet dropped", func(t *testing.T) {
		// Send a PCMA packet — should be dropped by the PCMU-only relay.
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
