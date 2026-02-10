package media

import (
	"log/slog"
	"net"
	"testing"
	"time"
)

func TestMediaSessionLifecycle(t *testing.T) {
	logger := slog.Default()

	proxy, err := NewProxy(19000, 19100, logger)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	mgr := NewSessionManager(proxy, logger)

	// Phase 1: Create — allocates ports, session in New state.
	ms, err := CreateMediaSession(mgr, "lifecycle-1", "call-lifecycle-1", logger)
	if err != nil {
		t.Fatalf("CreateMediaSession: %v", err)
	}

	if ms.State() != SessionStateNew {
		t.Fatalf("expected state New, got %s", ms.State())
	}

	if mgr.Count() != 1 {
		t.Fatalf("expected 1 session, got %d", mgr.Count())
	}

	if ms.CallerRTPPort() == 0 {
		t.Error("caller RTP port should be non-zero after allocation")
	}
	if ms.CalleeRTPPort() == 0 {
		t.Error("callee RTP port should be non-zero after allocation")
	}

	// No relay started yet — remote addresses should be nil.
	if ms.CallerAddr() != nil {
		t.Error("CallerAddr should be nil before relay starts")
	}
	if ms.CalleeAddr() != nil {
		t.Error("CalleeAddr should be nil before relay starts")
	}

	// Phase 2: Start relay.
	callerRemote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 50000}
	calleeRemote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 50002}

	err = ms.StartRelay(callerRemote, calleeRemote, []int{PayloadPCMA})
	if err != nil {
		t.Fatalf("StartRelay: %v", err)
	}

	if ms.State() != SessionStateActive {
		t.Fatalf("expected state Active, got %s", ms.State())
	}

	// Remote addresses should now be available.
	if ms.CallerAddr() == nil {
		t.Error("CallerAddr should be non-nil after relay starts")
	}
	if ms.CalleeAddr() == nil {
		t.Error("CalleeAddr should be non-nil after relay starts")
	}

	// Phase 3: Stop — session transitions to Stopped.
	ms.Stop()

	if ms.State() != SessionStateStopped {
		t.Fatalf("expected state Stopped, got %s", ms.State())
	}

	// Session still registered (not yet released).
	if mgr.Count() != 1 {
		t.Fatalf("expected 1 session (stopped but not released), got %d", mgr.Count())
	}

	// Phase 4: Release — ports returned to pool.
	ms.Release()

	if mgr.Count() != 0 {
		t.Fatalf("expected 0 sessions after release, got %d", mgr.Count())
	}
}

func TestMediaSessionReleaseWithoutStop(t *testing.T) {
	logger := slog.Default()

	proxy, err := NewProxy(19100, 19200, logger)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	mgr := NewSessionManager(proxy, logger)

	ms, err := CreateMediaSession(mgr, "lifecycle-2", "call-lifecycle-2", logger)
	if err != nil {
		t.Fatalf("CreateMediaSession: %v", err)
	}

	callerRemote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 50010}
	calleeRemote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 50012}

	err = ms.StartRelay(callerRemote, calleeRemote, []int{PayloadPCMU})
	if err != nil {
		t.Fatalf("StartRelay: %v", err)
	}

	if ms.State() != SessionStateActive {
		t.Fatalf("expected state Active, got %s", ms.State())
	}

	// Release directly without calling Stop first — should still clean up.
	ms.Release()

	if ms.State() != SessionStateStopped {
		t.Fatalf("expected state Stopped after release, got %s", ms.State())
	}

	if mgr.Count() != 0 {
		t.Fatalf("expected 0 sessions after release, got %d", mgr.Count())
	}
}

func TestMediaSessionReleaseWithoutRelay(t *testing.T) {
	logger := slog.Default()

	proxy, err := NewProxy(19200, 19300, logger)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	mgr := NewSessionManager(proxy, logger)

	ms, err := CreateMediaSession(mgr, "lifecycle-3", "call-lifecycle-3", logger)
	if err != nil {
		t.Fatalf("CreateMediaSession: %v", err)
	}

	if ms.State() != SessionStateNew {
		t.Fatalf("expected state New, got %s", ms.State())
	}

	// Release without ever starting a relay — should clean up ports.
	ms.Release()

	if mgr.Count() != 0 {
		t.Fatalf("expected 0 sessions after release, got %d", mgr.Count())
	}
}

func TestMediaSessionDoubleStartRelay(t *testing.T) {
	logger := slog.Default()

	proxy, err := NewProxy(19300, 19400, logger)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	mgr := NewSessionManager(proxy, logger)

	ms, err := CreateMediaSession(mgr, "lifecycle-4", "call-lifecycle-4", logger)
	if err != nil {
		t.Fatalf("CreateMediaSession: %v", err)
	}
	defer ms.Release()

	callerRemote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 50020}
	calleeRemote := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 50022}

	err = ms.StartRelay(callerRemote, calleeRemote, []int{PayloadPCMA})
	if err != nil {
		t.Fatalf("first StartRelay: %v", err)
	}

	// Starting relay again should fail.
	err = ms.StartRelay(callerRemote, calleeRemote, []int{PayloadPCMA})
	if err == nil {
		t.Fatal("expected error on double StartRelay, got nil")
	}
}

func TestMediaSessionRelayForwardsPackets(t *testing.T) {
	logger := slog.Default()

	proxy, err := NewProxy(19400, 19500, logger)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	mgr := NewSessionManager(proxy, logger)

	ms, err := CreateMediaSession(mgr, "lifecycle-fwd", "call-lifecycle-fwd", logger)
	if err != nil {
		t.Fatalf("CreateMediaSession: %v", err)
	}
	defer ms.Release()

	// Create phone endpoints.
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

	err = ms.StartRelay(callerPhoneAddr, calleePhoneAddr, []int{PayloadPCMA})
	if err != nil {
		t.Fatalf("StartRelay: %v", err)
	}

	// Send from caller phone → proxy caller leg → proxy callee leg → callee phone.
	callerLegAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: ms.CallerRTPPort()}
	pkt := makeTestRTPPacket(PayloadPCMA, []byte{0xAB, 0xCD, 0xEF})

	_, err = callerPhone.WriteToUDP(pkt, callerLegAddr)
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
	if rtpPayloadType(buf[:n]) != PayloadPCMA {
		t.Errorf("payload type = %d, want %d", rtpPayloadType(buf[:n]), PayloadPCMA)
	}
}

func TestMediaSessionDuplicateID(t *testing.T) {
	logger := slog.Default()

	proxy, err := NewProxy(19500, 19600, logger)
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	mgr := NewSessionManager(proxy, logger)

	ms1, err := CreateMediaSession(mgr, "dup-id", "call-dup", logger)
	if err != nil {
		t.Fatalf("first CreateMediaSession: %v", err)
	}
	defer ms1.Release()

	// Creating a second session with the same ID should fail.
	_, err = CreateMediaSession(mgr, "dup-id", "call-dup-2", logger)
	if err == nil {
		t.Fatal("expected error on duplicate session ID, got nil")
	}
}
