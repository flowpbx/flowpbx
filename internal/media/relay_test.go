package media

import (
	"bytes"
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

func TestOpusRelay(t *testing.T) {
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
		ID:        "test-session-opus",
		CallID:    "test-call-opus",
		CallerLeg: callerPair,
		CalleeLeg: calleePair,
		CreatedAt: time.Now(),
		state:     SessionStateNew,
	}

	relay := StartOpusRelay(session, callerPhoneAddr, calleePhoneAddr, logger)
	defer relay.Stop()

	if session.State() != SessionStateActive {
		t.Fatalf("expected session state Active, got %s", session.State())
	}

	t.Run("caller to callee Opus forwarded", func(t *testing.T) {
		pkt := makeTestRTPPacket(PayloadOpus, []byte{0x48, 0xC0, 0x01, 0x02})

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
		if rtpPayloadType(buf[:n]) != PayloadOpus {
			t.Errorf("payload type = %d, want %d", rtpPayloadType(buf[:n]), PayloadOpus)
		}
	})

	t.Run("callee to caller Opus forwarded", func(t *testing.T) {
		pkt := makeTestRTPPacket(PayloadOpus, []byte{0x78, 0x01, 0x33})

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
		if rtpPayloadType(buf[:n]) != PayloadOpus {
			t.Errorf("payload type = %d, want %d", rtpPayloadType(buf[:n]), PayloadOpus)
		}
	})

	t.Run("non-Opus packet dropped", func(t *testing.T) {
		// Send a PCMA packet — should be dropped by the Opus-only relay.
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

func TestSymmetricRTP_NAT(t *testing.T) {
	logger := slog.Default()

	// Create proxy socket pairs.
	callerPair, callerLocalAddr := allocateTestPair(t)
	defer callerPair.Close()
	calleePair, calleeLocalAddr := allocateTestPair(t)
	defer calleePair.Close()

	// Create the actual phone endpoints that will send/receive media.
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

	// Create a "wrong" SDP address to simulate NAT. The SDP advertises a
	// different address than where packets actually come from.
	bogusCallerAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 59999}
	bogusCalleeAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 59998}

	session := &Session{
		ID:        "test-session-nat",
		CallID:    "test-call-nat",
		CallerLeg: callerPair,
		CalleeLeg: calleePair,
		CreatedAt: time.Now(),
		state:     SessionStateNew,
	}

	// Start relay with bogus SDP addresses — symmetric RTP should learn the real ones.
	relay := StartPCMARelay(session, bogusCallerAddr, bogusCalleeAddr, logger)
	defer relay.Stop()

	// Verify initial addresses are the bogus SDP ones.
	if got := relay.CallerAddr(); got.Port != bogusCallerAddr.Port {
		t.Fatalf("initial caller addr = %s, want %s", got, bogusCallerAddr)
	}
	if got := relay.CalleeAddr(); got.Port != bogusCalleeAddr.Port {
		t.Fatalf("initial callee addr = %s, want %s", got, bogusCalleeAddr)
	}

	t.Run("learns caller address and forwards to callee", func(t *testing.T) {
		pkt := makeTestRTPPacket(PayloadPCMA, []byte{0xAA, 0xBB})

		// Caller phone sends to proxy's caller leg.
		// Since calleeRemote is bogus, this packet won't arrive at callee phone.
		// But the relay should learn the caller's real address.
		_, err := callerPhone.WriteToUDP(pkt, callerLocalAddr)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		// Allow time for the relay goroutine to process.
		time.Sleep(200 * time.Millisecond)

		// The relay should have learned the real caller address.
		learned := relay.CallerAddr()
		if learned.Port != callerPhoneAddr.Port {
			t.Errorf("learned caller port = %d, want %d", learned.Port, callerPhoneAddr.Port)
		}
	})

	t.Run("learns callee address and forwards to caller", func(t *testing.T) {
		pkt := makeTestRTPPacket(PayloadPCMA, []byte{0xCC, 0xDD})

		// Callee phone sends to proxy's callee leg.
		// Now callerRemote has been learned, so the relay should forward
		// this packet to the real caller phone.
		_, err := calleePhone.WriteToUDP(pkt, calleeLocalAddr)
		if err != nil {
			t.Fatalf("write: %v", err)
		}

		// Caller phone should receive the forwarded packet.
		callerPhone.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, maxRTPPacket)
		n, _, err := callerPhone.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("caller phone read: %v", err)
		}
		if !bytes.Equal(buf[:n], pkt) {
			t.Errorf("received packet differs from sent packet")
		}

		// The relay should also have learned the real callee address.
		learned := relay.CalleeAddr()
		if learned.Port != calleePhoneAddr.Port {
			t.Errorf("learned callee port = %d, want %d", learned.Port, calleePhoneAddr.Port)
		}
	})

	t.Run("bidirectional media flows to learned addresses", func(t *testing.T) {
		// Both addresses now learned. Verify caller→callee also works.
		pkt := makeTestRTPPacket(PayloadPCMA, []byte{0x11, 0x22, 0x33, 0x44})

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
		if !bytes.Equal(buf[:n], pkt) {
			t.Errorf("received packet differs from sent packet")
		}
	})
}

func TestSymmetricRTP_SameAddr(t *testing.T) {
	// When the actual source matches the SDP address, symmetric RTP should
	// not change anything — packets still flow correctly.
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
		ID:        "test-session-same",
		CallID:    "test-call-same",
		CallerLeg: callerPair,
		CalleeLeg: calleePair,
		CreatedAt: time.Now(),
		state:     SessionStateNew,
	}

	// Start with correct SDP addresses — no NAT scenario.
	relay := StartPCMURelay(session, callerPhoneAddr, calleePhoneAddr, logger)
	defer relay.Stop()

	// Caller → Callee.
	pkt := makeTestRTPPacket(PayloadPCMU, []byte{0xDE, 0xAD})
	_, err = callerPhone.WriteToUDP(pkt, callerLocalAddr)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	calleePhone.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, maxRTPPacket)
	n, _, err := calleePhone.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("callee phone read: %v", err)
	}
	if !bytes.Equal(buf[:n], pkt) {
		t.Errorf("received packet differs from sent packet")
	}

	// Callee → Caller.
	pkt2 := makeTestRTPPacket(PayloadPCMU, []byte{0xBE, 0xEF})
	_, err = calleePhone.WriteToUDP(pkt2, calleeLocalAddr)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	callerPhone.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err = callerPhone.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("caller phone read: %v", err)
	}
	if !bytes.Equal(buf[:n], pkt2) {
		t.Errorf("received packet differs from sent packet")
	}

	// Verify addresses unchanged — same as original SDP.
	if got := relay.CallerAddr(); got.Port != callerPhoneAddr.Port {
		t.Errorf("caller addr port = %d, want %d (should be unchanged)", got.Port, callerPhoneAddr.Port)
	}
	if got := relay.CalleeAddr(); got.Port != calleePhoneAddr.Port {
		t.Errorf("callee addr port = %d, want %d (should be unchanged)", got.Port, calleePhoneAddr.Port)
	}
}
