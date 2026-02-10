package media

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// SDP field type prefixes per RFC 4566.
const (
	sdpVersion    = "v="
	sdpOrigin     = "o="
	sdpSession    = "s="
	sdpConnection = "c="
	sdpTime       = "t="
	sdpMedia      = "m="
	sdpAttribute  = "a="
)

// Connection holds SDP connection data from a c= line.
// Format: c=<nettype> <addrtype> <connection-address>
type Connection struct {
	NetType  string // e.g. "IN"
	AddrType string // e.g. "IP4" or "IP6"
	Address  string // e.g. "192.168.1.10"
}

// String returns the SDP c= line value (without the "c=" prefix).
func (c Connection) String() string {
	return c.NetType + " " + c.AddrType + " " + c.Address
}

// Origin holds SDP origin data from an o= line.
// Format: o=<username> <sess-id> <sess-version> <nettype> <addrtype> <unicast-address>
type Origin struct {
	Username       string
	SessionID      string
	SessionVersion string
	NetType        string
	AddrType       string
	Address        string
}

// String returns the SDP o= line value (without the "o=" prefix).
func (o Origin) String() string {
	return o.Username + " " + o.SessionID + " " + o.SessionVersion + " " +
		o.NetType + " " + o.AddrType + " " + o.Address
}

// Codec represents a codec from an SDP rtpmap attribute.
type Codec struct {
	PayloadType int    // RTP payload type number
	Name        string // codec name, e.g. "PCMU", "PCMA", "opus"
	ClockRate   int    // clock rate in Hz
	Channels    int    // number of channels (0 means not specified, defaults to 1)
	Fmtp        string // format parameters from a=fmtp line, if any
}

// String returns the rtpmap attribute value.
func (c Codec) String() string {
	s := strconv.Itoa(c.PayloadType) + " " + c.Name + "/" + strconv.Itoa(c.ClockRate)
	if c.Channels > 0 {
		s += "/" + strconv.Itoa(c.Channels)
	}
	return s
}

// MediaDescription holds a parsed SDP m= section with its attributes.
type MediaDescription struct {
	Type       string      // "audio", "video", etc.
	Port       int         // transport port
	NumPorts   int         // number of ports (0 means 1)
	Proto      string      // e.g. "RTP/AVP", "RTP/SAVP"
	Formats    []int       // payload type numbers
	Connection *Connection // media-level c= line (overrides session-level)
	Codecs     []Codec     // parsed from a=rtpmap lines
	Attributes []string    // raw a= lines for this media section
	Direction  string      // "sendrecv", "sendonly", "recvonly", "inactive"
}

// CodecByPayloadType returns the codec with the given payload type, or nil.
func (m *MediaDescription) CodecByPayloadType(pt int) *Codec {
	for i := range m.Codecs {
		if m.Codecs[i].PayloadType == pt {
			return &m.Codecs[i]
		}
	}
	return nil
}

// CodecByName returns the first codec with the given name (case-insensitive), or nil.
func (m *MediaDescription) CodecByName(name string) *Codec {
	lower := strings.ToLower(name)
	for i := range m.Codecs {
		if strings.ToLower(m.Codecs[i].Name) == lower {
			return &m.Codecs[i]
		}
	}
	return nil
}

// HasCodec returns true if the media description includes a codec with the given name.
func (m *MediaDescription) HasCodec(name string) bool {
	return m.CodecByName(name) != nil
}

// SessionDescription holds a fully parsed SDP session.
type SessionDescription struct {
	Version     int
	Origin      Origin
	SessionName string
	Connection  *Connection // session-level c= line
	Time        string      // t= line value
	Media       []MediaDescription
	Attributes  []string // session-level a= lines
	RawLines    []string // original lines for reconstruction
}

// AudioMedia returns the first audio media description, or nil if none.
func (s *SessionDescription) AudioMedia() *MediaDescription {
	for i := range s.Media {
		if s.Media[i].Type == "audio" {
			return &s.Media[i]
		}
	}
	return nil
}

// ConnectionAddress returns the effective connection address for a media
// description, preferring the media-level c= line over the session-level one.
func (s *SessionDescription) ConnectionAddress(m *MediaDescription) string {
	if m.Connection != nil {
		return m.Connection.Address
	}
	if s.Connection != nil {
		return s.Connection.Address
	}
	return ""
}

// ParseSDP parses an SDP body into a SessionDescription.
func ParseSDP(data []byte) (*SessionDescription, error) {
	text := string(data)
	// Normalize line endings: SDP uses \r\n per spec, but handle \n too.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	lines := strings.Split(text, "\n")

	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil, fmt.Errorf("empty sdp body")
	}

	sd := &SessionDescription{
		RawLines: lines,
	}

	var currentMedia *MediaDescription

	for _, line := range lines {
		if len(line) < 2 || line[1] != '=' {
			continue // skip malformed lines
		}

		switch {
		case strings.HasPrefix(line, sdpVersion):
			v, err := strconv.Atoi(line[2:])
			if err != nil {
				return nil, fmt.Errorf("invalid sdp version: %w", err)
			}
			sd.Version = v

		case strings.HasPrefix(line, sdpOrigin):
			origin, err := parseOrigin(line[2:])
			if err != nil {
				return nil, fmt.Errorf("invalid sdp origin: %w", err)
			}
			sd.Origin = origin

		case strings.HasPrefix(line, sdpSession):
			sd.SessionName = line[2:]

		case strings.HasPrefix(line, sdpConnection):
			conn, err := parseConnection(line[2:])
			if err != nil {
				return nil, fmt.Errorf("invalid sdp connection: %w", err)
			}
			if currentMedia != nil {
				currentMedia.Connection = &conn
			} else {
				sd.Connection = &conn
			}

		case strings.HasPrefix(line, sdpTime):
			sd.Time = line[2:]

		case strings.HasPrefix(line, sdpMedia):
			md, err := parseMediaLine(line[2:])
			if err != nil {
				return nil, fmt.Errorf("invalid sdp media line: %w", err)
			}
			sd.Media = append(sd.Media, md)
			currentMedia = &sd.Media[len(sd.Media)-1]

		case strings.HasPrefix(line, sdpAttribute):
			attr := line[2:]
			if currentMedia != nil {
				currentMedia.Attributes = append(currentMedia.Attributes, attr)
				parseMediaAttribute(currentMedia, attr)
			} else {
				sd.Attributes = append(sd.Attributes, attr)
			}
		}
	}

	return sd, nil
}

// Marshal serializes a SessionDescription back to SDP format.
func (s *SessionDescription) Marshal() []byte {
	var b strings.Builder

	b.WriteString("v=" + strconv.Itoa(s.Version) + "\r\n")
	b.WriteString("o=" + s.Origin.String() + "\r\n")
	b.WriteString("s=" + s.SessionName + "\r\n")

	if s.Connection != nil {
		b.WriteString("c=" + s.Connection.String() + "\r\n")
	}

	b.WriteString("t=" + s.Time + "\r\n")

	for _, attr := range s.Attributes {
		b.WriteString("a=" + attr + "\r\n")
	}

	for _, m := range s.Media {
		// Build m= line
		fmts := make([]string, len(m.Formats))
		for i, f := range m.Formats {
			fmts[i] = strconv.Itoa(f)
		}
		portStr := strconv.Itoa(m.Port)
		if m.NumPorts > 0 {
			portStr += "/" + strconv.Itoa(m.NumPorts)
		}
		b.WriteString("m=" + m.Type + " " + portStr + " " + m.Proto + " " + strings.Join(fmts, " ") + "\r\n")

		if m.Connection != nil {
			b.WriteString("c=" + m.Connection.String() + "\r\n")
		}

		for _, attr := range m.Attributes {
			b.WriteString("a=" + attr + "\r\n")
		}
	}

	return []byte(b.String())
}

// parseConnection parses a connection data value: <nettype> <addrtype> <address>
func parseConnection(value string) (Connection, error) {
	parts := strings.Fields(value)
	if len(parts) < 3 {
		return Connection{}, fmt.Errorf("expected 3 fields, got %d", len(parts))
	}

	addr := parts[2]
	// Strip TTL/multicast suffix if present (e.g. "224.2.1.1/127")
	if idx := strings.Index(addr, "/"); idx >= 0 {
		addr = addr[:idx]
	}

	if net.ParseIP(addr) == nil {
		return Connection{}, fmt.Errorf("invalid ip address %q", addr)
	}

	return Connection{
		NetType:  parts[0],
		AddrType: parts[1],
		Address:  addr,
	}, nil
}

// parseOrigin parses an origin value:
// <username> <sess-id> <sess-version> <nettype> <addrtype> <unicast-address>
func parseOrigin(value string) (Origin, error) {
	parts := strings.Fields(value)
	if len(parts) < 6 {
		return Origin{}, fmt.Errorf("expected 6 fields, got %d", len(parts))
	}
	return Origin{
		Username:       parts[0],
		SessionID:      parts[1],
		SessionVersion: parts[2],
		NetType:        parts[3],
		AddrType:       parts[4],
		Address:        parts[5],
	}, nil
}

// parseMediaLine parses a media description line value:
// <media> <port>[/<number of ports>] <proto> <fmt> ...
func parseMediaLine(value string) (MediaDescription, error) {
	parts := strings.Fields(value)
	if len(parts) < 4 {
		return MediaDescription{}, fmt.Errorf("expected at least 4 fields, got %d", len(parts))
	}

	md := MediaDescription{
		Type:      parts[0],
		Proto:     parts[2],
		Direction: "sendrecv", // default per RFC 3264
	}

	// Parse port, possibly with /numPorts
	portStr := parts[1]
	if idx := strings.Index(portStr, "/"); idx >= 0 {
		numPorts, err := strconv.Atoi(portStr[idx+1:])
		if err != nil {
			return MediaDescription{}, fmt.Errorf("invalid port count: %w", err)
		}
		md.NumPorts = numPorts
		portStr = portStr[:idx]
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return MediaDescription{}, fmt.Errorf("invalid port: %w", err)
	}
	md.Port = port

	// Parse format list (payload types)
	for _, fmtStr := range parts[3:] {
		pt, err := strconv.Atoi(fmtStr)
		if err != nil {
			return MediaDescription{}, fmt.Errorf("invalid payload type %q: %w", fmtStr, err)
		}
		md.Formats = append(md.Formats, pt)
	}

	return md, nil
}

// parseMediaAttribute processes a single attribute for a media description.
func parseMediaAttribute(md *MediaDescription, attr string) {
	switch {
	case strings.HasPrefix(attr, "rtpmap:"):
		codec, err := parseRtpmap(attr[7:])
		if err == nil {
			// Attach fmtp if we already have one for this PT
			for i := range md.Codecs {
				if md.Codecs[i].PayloadType == codec.PayloadType {
					codec.Fmtp = md.Codecs[i].Fmtp
					md.Codecs[i] = codec
					return
				}
			}
			md.Codecs = append(md.Codecs, codec)
		}

	case strings.HasPrefix(attr, "fmtp:"):
		pt, params, ok := parseFmtp(attr[5:])
		if ok {
			// Attach to existing codec or create placeholder
			for i := range md.Codecs {
				if md.Codecs[i].PayloadType == pt {
					md.Codecs[i].Fmtp = params
					return
				}
			}
			// fmtp arrived before rtpmap; store as placeholder
			md.Codecs = append(md.Codecs, Codec{PayloadType: pt, Fmtp: params})
		}

	case attr == "sendrecv" || attr == "sendonly" || attr == "recvonly" || attr == "inactive":
		md.Direction = attr
	}
}

// parseRtpmap parses an rtpmap attribute value:
// <payload type> <encoding name>/<clock rate>[/<channels>]
func parseRtpmap(value string) (Codec, error) {
	parts := strings.SplitN(value, " ", 2)
	if len(parts) != 2 {
		return Codec{}, fmt.Errorf("expected '<pt> <encoding>', got %q", value)
	}

	pt, err := strconv.Atoi(parts[0])
	if err != nil {
		return Codec{}, fmt.Errorf("invalid payload type: %w", err)
	}

	encParts := strings.Split(parts[1], "/")
	if len(encParts) < 2 {
		return Codec{}, fmt.Errorf("expected '<name>/<rate>', got %q", parts[1])
	}

	clockRate, err := strconv.Atoi(encParts[1])
	if err != nil {
		return Codec{}, fmt.Errorf("invalid clock rate: %w", err)
	}

	codec := Codec{
		PayloadType: pt,
		Name:        encParts[0],
		ClockRate:   clockRate,
	}

	if len(encParts) >= 3 {
		ch, err := strconv.Atoi(encParts[2])
		if err == nil {
			codec.Channels = ch
		}
	}

	return codec, nil
}

// parseFmtp parses an fmtp attribute value: <payload type> <params>
func parseFmtp(value string) (int, string, bool) {
	parts := strings.SplitN(value, " ", 2)
	if len(parts) < 2 {
		return 0, "", false
	}
	pt, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}
	return pt, parts[1], true
}
