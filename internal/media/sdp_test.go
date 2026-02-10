package media

import (
	"strings"
	"testing"
)

// Typical SDP offer from a SIP phone with audio codecs.
const testSDPOffer = `v=0
o=alice 2890844526 2890844526 IN IP4 192.168.1.100
s=Phone Call
c=IN IP4 192.168.1.100
t=0 0
m=audio 49170 RTP/AVP 0 8 111 101
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:111 opus/48000/2
a=fmtp:111 minptime=10;useinbandfec=1
a=rtpmap:101 telephone-event/8000
a=fmtp:101 0-16
a=sendrecv
`

func TestParseSDP(t *testing.T) {
	sd, err := ParseSDP([]byte(testSDPOffer))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	if sd.Version != 0 {
		t.Errorf("version = %d, want 0", sd.Version)
	}

	if sd.Origin.Username != "alice" {
		t.Errorf("origin username = %q, want %q", sd.Origin.Username, "alice")
	}
	if sd.Origin.Address != "192.168.1.100" {
		t.Errorf("origin address = %q, want %q", sd.Origin.Address, "192.168.1.100")
	}

	if sd.SessionName != "Phone Call" {
		t.Errorf("session name = %q, want %q", sd.SessionName, "Phone Call")
	}

	if sd.Connection == nil {
		t.Fatal("session-level connection is nil")
	}
	if sd.Connection.Address != "192.168.1.100" {
		t.Errorf("connection address = %q, want %q", sd.Connection.Address, "192.168.1.100")
	}
	if sd.Connection.AddrType != "IP4" {
		t.Errorf("connection addr type = %q, want %q", sd.Connection.AddrType, "IP4")
	}

	if len(sd.Media) != 1 {
		t.Fatalf("media count = %d, want 1", len(sd.Media))
	}

	m := sd.Media[0]
	if m.Type != "audio" {
		t.Errorf("media type = %q, want %q", m.Type, "audio")
	}
	if m.Port != 49170 {
		t.Errorf("media port = %d, want 49170", m.Port)
	}
	if m.Proto != "RTP/AVP" {
		t.Errorf("media proto = %q, want %q", m.Proto, "RTP/AVP")
	}

	// Check payload types
	wantFormats := []int{0, 8, 111, 101}
	if len(m.Formats) != len(wantFormats) {
		t.Fatalf("format count = %d, want %d", len(m.Formats), len(wantFormats))
	}
	for i, f := range wantFormats {
		if m.Formats[i] != f {
			t.Errorf("format[%d] = %d, want %d", i, m.Formats[i], f)
		}
	}

	// Check codecs parsed from rtpmap
	if len(m.Codecs) != 4 {
		t.Fatalf("codec count = %d, want 4", len(m.Codecs))
	}

	pcmu := m.CodecByPayloadType(0)
	if pcmu == nil {
		t.Fatal("PCMU codec not found")
	}
	if pcmu.Name != "PCMU" || pcmu.ClockRate != 8000 {
		t.Errorf("PCMU = %+v", pcmu)
	}

	pcma := m.CodecByPayloadType(8)
	if pcma == nil {
		t.Fatal("PCMA codec not found")
	}
	if pcma.Name != "PCMA" || pcma.ClockRate != 8000 {
		t.Errorf("PCMA = %+v", pcma)
	}

	opus := m.CodecByPayloadType(111)
	if opus == nil {
		t.Fatal("opus codec not found")
	}
	if opus.Name != "opus" || opus.ClockRate != 48000 || opus.Channels != 2 {
		t.Errorf("opus = %+v", opus)
	}
	if opus.Fmtp != "minptime=10;useinbandfec=1" {
		t.Errorf("opus fmtp = %q, want %q", opus.Fmtp, "minptime=10;useinbandfec=1")
	}

	te := m.CodecByPayloadType(101)
	if te == nil {
		t.Fatal("telephone-event codec not found")
	}
	if te.Name != "telephone-event" || te.ClockRate != 8000 {
		t.Errorf("telephone-event = %+v", te)
	}
	if te.Fmtp != "0-16" {
		t.Errorf("telephone-event fmtp = %q, want %q", te.Fmtp, "0-16")
	}

	if m.Direction != "sendrecv" {
		t.Errorf("direction = %q, want %q", m.Direction, "sendrecv")
	}
}

func TestParseSDP_CRLF(t *testing.T) {
	// SDP per RFC should use \r\n
	sdp := "v=0\r\no=- 1 1 IN IP4 10.0.0.1\r\ns=-\r\nc=IN IP4 10.0.0.1\r\nt=0 0\r\nm=audio 5004 RTP/AVP 0\r\na=rtpmap:0 PCMU/8000\r\n"
	sd, err := ParseSDP([]byte(sdp))
	if err != nil {
		t.Fatalf("ParseSDP with CRLF failed: %v", err)
	}
	if len(sd.Media) != 1 {
		t.Fatalf("media count = %d, want 1", len(sd.Media))
	}
	if sd.Media[0].Port != 5004 {
		t.Errorf("port = %d, want 5004", sd.Media[0].Port)
	}
}

func TestParseSDP_MediaLevelConnection(t *testing.T) {
	sdp := `v=0
o=- 1 1 IN IP4 10.0.0.1
s=-
c=IN IP4 10.0.0.1
t=0 0
m=audio 5004 RTP/AVP 0
c=IN IP4 172.16.0.5
a=rtpmap:0 PCMU/8000
`
	sd, err := ParseSDP([]byte(sdp))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	if sd.Connection.Address != "10.0.0.1" {
		t.Errorf("session connection = %q, want %q", sd.Connection.Address, "10.0.0.1")
	}

	m := sd.Media[0]
	if m.Connection == nil {
		t.Fatal("media-level connection is nil")
	}
	if m.Connection.Address != "172.16.0.5" {
		t.Errorf("media connection = %q, want %q", m.Connection.Address, "172.16.0.5")
	}

	// ConnectionAddress should prefer media-level
	addr := sd.ConnectionAddress(&m)
	if addr != "172.16.0.5" {
		t.Errorf("ConnectionAddress = %q, want %q", addr, "172.16.0.5")
	}
}

func TestParseSDP_MultipleMedia(t *testing.T) {
	sdp := `v=0
o=- 1 1 IN IP4 10.0.0.1
s=-
c=IN IP4 10.0.0.1
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
m=video 5006 RTP/AVP 96
a=rtpmap:96 H264/90000
`
	sd, err := ParseSDP([]byte(sdp))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	if len(sd.Media) != 2 {
		t.Fatalf("media count = %d, want 2", len(sd.Media))
	}

	if sd.Media[0].Type != "audio" {
		t.Errorf("media[0].Type = %q, want %q", sd.Media[0].Type, "audio")
	}
	if sd.Media[1].Type != "video" {
		t.Errorf("media[1].Type = %q, want %q", sd.Media[1].Type, "video")
	}
}

func TestParseSDP_FmtpBeforeRtpmap(t *testing.T) {
	// Some endpoints send fmtp before rtpmap
	sdp := `v=0
o=- 1 1 IN IP4 10.0.0.1
s=-
c=IN IP4 10.0.0.1
t=0 0
m=audio 5004 RTP/AVP 111
a=fmtp:111 minptime=10
a=rtpmap:111 opus/48000/2
`
	sd, err := ParseSDP([]byte(sdp))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	m := sd.Media[0]
	opus := m.CodecByPayloadType(111)
	if opus == nil {
		t.Fatal("opus codec not found")
	}
	if opus.Name != "opus" {
		t.Errorf("codec name = %q, want %q", opus.Name, "opus")
	}
	if opus.Fmtp != "minptime=10" {
		t.Errorf("fmtp = %q, want %q", opus.Fmtp, "minptime=10")
	}
}

func TestParseSDP_Direction(t *testing.T) {
	tests := []struct {
		dir  string
		want string
	}{
		{"sendrecv", "sendrecv"},
		{"sendonly", "sendonly"},
		{"recvonly", "recvonly"},
		{"inactive", "inactive"},
	}

	for _, tt := range tests {
		sdp := `v=0
o=- 1 1 IN IP4 10.0.0.1
s=-
c=IN IP4 10.0.0.1
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
a=` + tt.dir + "\n"

		sd, err := ParseSDP([]byte(sdp))
		if err != nil {
			t.Fatalf("ParseSDP(%q) failed: %v", tt.dir, err)
		}
		if sd.Media[0].Direction != tt.want {
			t.Errorf("direction for %q = %q, want %q", tt.dir, sd.Media[0].Direction, tt.want)
		}
	}
}

func TestParseSDP_DefaultDirection(t *testing.T) {
	// If no direction attribute, default is sendrecv per RFC 3264
	sdp := `v=0
o=- 1 1 IN IP4 10.0.0.1
s=-
c=IN IP4 10.0.0.1
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`
	sd, err := ParseSDP([]byte(sdp))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}
	if sd.Media[0].Direction != "sendrecv" {
		t.Errorf("default direction = %q, want %q", sd.Media[0].Direction, "sendrecv")
	}
}

func TestParseSDP_Empty(t *testing.T) {
	_, err := ParseSDP([]byte(""))
	if err == nil {
		t.Error("expected error for empty SDP")
	}
}

func TestParseSDP_PortRange(t *testing.T) {
	sdp := `v=0
o=- 1 1 IN IP4 10.0.0.1
s=-
c=IN IP4 10.0.0.1
t=0 0
m=audio 5004/2 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`
	sd, err := ParseSDP([]byte(sdp))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}
	if sd.Media[0].Port != 5004 {
		t.Errorf("port = %d, want 5004", sd.Media[0].Port)
	}
	if sd.Media[0].NumPorts != 2 {
		t.Errorf("num_ports = %d, want 2", sd.Media[0].NumPorts)
	}
}

func TestAudioMedia(t *testing.T) {
	sd, err := ParseSDP([]byte(testSDPOffer))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	audio := sd.AudioMedia()
	if audio == nil {
		t.Fatal("AudioMedia returned nil")
	}
	if audio.Type != "audio" {
		t.Errorf("AudioMedia type = %q, want %q", audio.Type, "audio")
	}
}

func TestCodecByName(t *testing.T) {
	sd, err := ParseSDP([]byte(testSDPOffer))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	m := sd.AudioMedia()

	// Case-insensitive lookup
	if c := m.CodecByName("pcmu"); c == nil {
		t.Error("CodecByName(pcmu) returned nil")
	}
	if c := m.CodecByName("PCMU"); c == nil {
		t.Error("CodecByName(PCMU) returned nil")
	}
	if c := m.CodecByName("nonexistent"); c != nil {
		t.Error("CodecByName(nonexistent) should return nil")
	}

	if !m.HasCodec("opus") {
		t.Error("HasCodec(opus) should be true")
	}
	if m.HasCodec("G729") {
		t.Error("HasCodec(G729) should be false")
	}
}

func TestMarshalSDP(t *testing.T) {
	sd, err := ParseSDP([]byte(testSDPOffer))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	out := sd.Marshal()

	// Re-parse the marshaled output to verify round-trip
	sd2, err := ParseSDP(out)
	if err != nil {
		t.Fatalf("ParseSDP(marshaled) failed: %v", err)
	}

	if sd2.Version != sd.Version {
		t.Errorf("version mismatch: %d vs %d", sd2.Version, sd.Version)
	}
	if sd2.Origin.Username != sd.Origin.Username {
		t.Errorf("origin username mismatch: %q vs %q", sd2.Origin.Username, sd.Origin.Username)
	}
	if sd2.Connection.Address != sd.Connection.Address {
		t.Errorf("connection address mismatch: %q vs %q", sd2.Connection.Address, sd.Connection.Address)
	}
	if len(sd2.Media) != len(sd.Media) {
		t.Fatalf("media count mismatch: %d vs %d", len(sd2.Media), len(sd.Media))
	}

	m1 := sd.Media[0]
	m2 := sd2.Media[0]
	if m2.Port != m1.Port {
		t.Errorf("media port mismatch: %d vs %d", m2.Port, m1.Port)
	}
	if len(m2.Codecs) != len(m1.Codecs) {
		t.Errorf("codec count mismatch: %d vs %d", len(m2.Codecs), len(m1.Codecs))
	}

	// Verify CRLF line endings
	if !strings.Contains(string(out), "\r\n") {
		t.Error("marshaled SDP should use CRLF line endings")
	}
}

func TestConnectionString(t *testing.T) {
	c := Connection{NetType: "IN", AddrType: "IP4", Address: "10.0.0.1"}
	if s := c.String(); s != "IN IP4 10.0.0.1" {
		t.Errorf("Connection.String() = %q, want %q", s, "IN IP4 10.0.0.1")
	}
}

func TestOriginString(t *testing.T) {
	o := Origin{
		Username:       "alice",
		SessionID:      "123",
		SessionVersion: "456",
		NetType:        "IN",
		AddrType:       "IP4",
		Address:        "10.0.0.1",
	}
	want := "alice 123 456 IN IP4 10.0.0.1"
	if s := o.String(); s != want {
		t.Errorf("Origin.String() = %q, want %q", s, want)
	}
}

func TestCodecString(t *testing.T) {
	c := Codec{PayloadType: 111, Name: "opus", ClockRate: 48000, Channels: 2}
	want := "111 opus/48000/2"
	if s := c.String(); s != want {
		t.Errorf("Codec.String() = %q, want %q", s, want)
	}

	c2 := Codec{PayloadType: 0, Name: "PCMU", ClockRate: 8000}
	want2 := "0 PCMU/8000"
	if s := c2.String(); s != want2 {
		t.Errorf("Codec.String() = %q, want %q", s, want2)
	}
}

func TestRewriteSDP(t *testing.T) {
	sd, err := ParseSDP([]byte(testSDPOffer))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	rewritten := RewriteSDP(sd, "203.0.113.5", 20000)

	// Origin should be updated.
	if rewritten.Origin.Address != "203.0.113.5" {
		t.Errorf("origin address = %q, want %q", rewritten.Origin.Address, "203.0.113.5")
	}

	// Session-level connection should be updated.
	if rewritten.Connection == nil {
		t.Fatal("session-level connection is nil")
	}
	if rewritten.Connection.Address != "203.0.113.5" {
		t.Errorf("connection address = %q, want %q", rewritten.Connection.Address, "203.0.113.5")
	}
	if rewritten.Connection.AddrType != "IP4" {
		t.Errorf("connection addr type = %q, want %q", rewritten.Connection.AddrType, "IP4")
	}

	// Audio media port should be updated.
	audio := rewritten.AudioMedia()
	if audio == nil {
		t.Fatal("audio media not found")
	}
	if audio.Port != 20000 {
		t.Errorf("audio port = %d, want 20000", audio.Port)
	}

	// Original should not be modified.
	if sd.Connection.Address != "192.168.1.100" {
		t.Errorf("original connection address modified: %q", sd.Connection.Address)
	}
	if sd.Origin.Address != "192.168.1.100" {
		t.Errorf("original origin address modified: %q", sd.Origin.Address)
	}
	origAudio := sd.AudioMedia()
	if origAudio.Port != 49170 {
		t.Errorf("original audio port modified: %d", origAudio.Port)
	}
}

func TestRewriteSDP_MediaLevelConnection(t *testing.T) {
	sdp := `v=0
o=- 1 1 IN IP4 10.0.0.1
s=-
c=IN IP4 10.0.0.1
t=0 0
m=audio 5004 RTP/AVP 0
c=IN IP4 172.16.0.5
a=rtpmap:0 PCMU/8000
`
	sd, err := ParseSDP([]byte(sdp))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	rewritten := RewriteSDP(sd, "203.0.113.5", 30000)

	// Session-level connection should be updated.
	if rewritten.Connection.Address != "203.0.113.5" {
		t.Errorf("session connection = %q, want %q", rewritten.Connection.Address, "203.0.113.5")
	}

	// Media-level connection should also be updated.
	m := rewritten.Media[0]
	if m.Connection == nil {
		t.Fatal("media-level connection is nil")
	}
	if m.Connection.Address != "203.0.113.5" {
		t.Errorf("media connection = %q, want %q", m.Connection.Address, "203.0.113.5")
	}
	if m.Port != 30000 {
		t.Errorf("media port = %d, want 30000", m.Port)
	}

	// Original media connection should be untouched.
	if sd.Media[0].Connection.Address != "172.16.0.5" {
		t.Errorf("original media connection modified: %q", sd.Media[0].Connection.Address)
	}
}

func TestRewriteSDP_IPv6Proxy(t *testing.T) {
	sd, err := ParseSDP([]byte(testSDPOffer))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	rewritten := RewriteSDP(sd, "2001:db8::1", 20000)

	if rewritten.Connection.Address != "2001:db8::1" {
		t.Errorf("connection address = %q, want %q", rewritten.Connection.Address, "2001:db8::1")
	}
	if rewritten.Connection.AddrType != "IP6" {
		t.Errorf("connection addr type = %q, want %q", rewritten.Connection.AddrType, "IP6")
	}
}

func TestRewriteSDP_NonAudioNotRewritten(t *testing.T) {
	sdp := `v=0
o=- 1 1 IN IP4 10.0.0.1
s=-
c=IN IP4 10.0.0.1
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
m=video 5006 RTP/AVP 96
a=rtpmap:96 H264/90000
`
	sd, err := ParseSDP([]byte(sdp))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	rewritten := RewriteSDP(sd, "203.0.113.5", 30000)

	// Audio port should be rewritten.
	if rewritten.Media[0].Port != 30000 {
		t.Errorf("audio port = %d, want 30000", rewritten.Media[0].Port)
	}

	// Video port should be unchanged (only audio is rewritten).
	if rewritten.Media[1].Port != 5006 {
		t.Errorf("video port = %d, want 5006 (unchanged)", rewritten.Media[1].Port)
	}
}

func TestRewriteSDPBytes(t *testing.T) {
	result, err := RewriteSDPBytes([]byte(testSDPOffer), "203.0.113.5", 20000)
	if err != nil {
		t.Fatalf("RewriteSDPBytes failed: %v", err)
	}

	// Re-parse the result to verify.
	sd, err := ParseSDP(result)
	if err != nil {
		t.Fatalf("ParseSDP(rewritten) failed: %v", err)
	}

	if sd.Connection.Address != "203.0.113.5" {
		t.Errorf("connection address = %q, want %q", sd.Connection.Address, "203.0.113.5")
	}
	if sd.Origin.Address != "203.0.113.5" {
		t.Errorf("origin address = %q, want %q", sd.Origin.Address, "203.0.113.5")
	}

	audio := sd.AudioMedia()
	if audio.Port != 20000 {
		t.Errorf("audio port = %d, want 20000", audio.Port)
	}

	// Should use CRLF line endings.
	if !strings.Contains(string(result), "\r\n") {
		t.Error("rewritten SDP should use CRLF line endings")
	}
}

func TestRewriteSDPBytes_InvalidInput(t *testing.T) {
	_, err := RewriteSDPBytes([]byte(""), "203.0.113.5", 20000)
	if err == nil {
		t.Error("expected error for empty SDP input")
	}
}

func TestRewriteSDP_PreservesCodecs(t *testing.T) {
	sd, err := ParseSDP([]byte(testSDPOffer))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	rewritten := RewriteSDP(sd, "203.0.113.5", 20000)

	audio := rewritten.AudioMedia()
	if audio == nil {
		t.Fatal("no audio media")
	}

	// All codecs should be preserved.
	if len(audio.Codecs) != 4 {
		t.Fatalf("codec count = %d, want 4", len(audio.Codecs))
	}
	if !audio.HasCodec("PCMU") {
		t.Error("missing PCMU codec")
	}
	if !audio.HasCodec("PCMA") {
		t.Error("missing PCMA codec")
	}
	if !audio.HasCodec("opus") {
		t.Error("missing opus codec")
	}
	if !audio.HasCodec("telephone-event") {
		t.Error("missing telephone-event codec")
	}

	// Direction should be preserved.
	if audio.Direction != "sendrecv" {
		t.Errorf("direction = %q, want %q", audio.Direction, "sendrecv")
	}
}

func TestRewriteSDP_NoSessionConnection(t *testing.T) {
	// SDP with no session-level connection (only media-level)
	sdp := `v=0
o=- 1 1 IN IP4 10.0.0.1
s=-
t=0 0
m=audio 5004 RTP/AVP 0
c=IN IP4 172.16.0.5
a=rtpmap:0 PCMU/8000
`
	sd, err := ParseSDP([]byte(sdp))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}

	rewritten := RewriteSDP(sd, "203.0.113.5", 30000)

	// Session-level connection should remain nil.
	if rewritten.Connection != nil {
		t.Error("session-level connection should remain nil")
	}

	// Media-level connection should be updated.
	if rewritten.Media[0].Connection.Address != "203.0.113.5" {
		t.Errorf("media connection = %q, want %q", rewritten.Media[0].Connection.Address, "203.0.113.5")
	}
	if rewritten.Media[0].Port != 30000 {
		t.Errorf("media port = %d, want 30000", rewritten.Media[0].Port)
	}
}

func TestParseSDP_IPv6(t *testing.T) {
	sdp := `v=0
o=- 1 1 IN IP6 2001:db8::1
s=-
c=IN IP6 2001:db8::1
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`
	sd, err := ParseSDP([]byte(sdp))
	if err != nil {
		t.Fatalf("ParseSDP failed: %v", err)
	}
	if sd.Connection.AddrType != "IP6" {
		t.Errorf("addr type = %q, want %q", sd.Connection.AddrType, "IP6")
	}
	if sd.Connection.Address != "2001:db8::1" {
		t.Errorf("address = %q, want %q", sd.Connection.Address, "2001:db8::1")
	}
}
