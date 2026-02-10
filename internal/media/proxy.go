package media

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
)

// PortPair holds an RTP port and its companion RTCP port (RTP+1).
type PortPair struct {
	RTP  int
	RTCP int
}

// SocketPair holds the UDP connections for an RTP/RTCP port pair.
type SocketPair struct {
	Ports    PortPair
	RTPConn  *net.UDPConn
	RTCPConn *net.UDPConn
}

// Close releases both UDP sockets.
func (sp *SocketPair) Close() error {
	var rtpErr, rtcpErr error
	if sp.RTPConn != nil {
		rtpErr = sp.RTPConn.Close()
	}
	if sp.RTCPConn != nil {
		rtcpErr = sp.RTCPConn.Close()
	}
	if rtpErr != nil {
		return rtpErr
	}
	return rtcpErr
}

// Proxy manages a pool of UDP sockets for RTP media relay. It allocates
// even-numbered ports for RTP and the next odd port for RTCP, within a
// configurable range.
type Proxy struct {
	portMin int
	portMax int
	logger  *slog.Logger

	mu        sync.Mutex
	allocated map[int]struct{} // set of allocated RTP ports (even numbers)
	nextPort  int              // next port to try (even number)
}

// NewProxy creates an RTP media proxy with the given port range.
// portMin must be even; portMax must be > portMin.
func NewProxy(portMin, portMax int, logger *slog.Logger) (*Proxy, error) {
	if portMin%2 != 0 {
		return nil, fmt.Errorf("portMin must be even, got %d", portMin)
	}
	if portMax <= portMin {
		return nil, fmt.Errorf("portMax (%d) must be greater than portMin (%d)", portMax, portMin)
	}

	l := logger.With("subsystem", "media-proxy")
	capacity := (portMax - portMin + 1) / 2
	l.Info("rtp media proxy initialized",
		"port_min", portMin,
		"port_max", portMax,
		"capacity", capacity,
	)

	return &Proxy{
		portMin:   portMin,
		portMax:   portMax,
		logger:    l,
		allocated: make(map[int]struct{}),
		nextPort:  portMin,
	}, nil
}

// Capacity returns the total number of port pairs available in the range.
func (p *Proxy) Capacity() int {
	return (p.portMax - p.portMin + 1) / 2
}

// AllocatedCount returns the number of currently allocated port pairs.
func (p *Proxy) AllocatedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.allocated)
}

// Allocate binds an RTP+RTCP UDP socket pair from the port pool.
// It returns a SocketPair with both connections ready for use.
// Returns an error if no ports are available or binding fails.
func (p *Proxy) Allocate() (*SocketPair, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	capacity := (p.portMax - p.portMin + 1) / 2
	if len(p.allocated) >= capacity {
		return nil, fmt.Errorf("no rtp ports available (all %d pairs allocated)", capacity)
	}

	// Scan from nextPort through the range to find an available even port.
	startPort := p.nextPort
	for {
		port := p.nextPort

		// Advance nextPort for the next allocation (wrap around).
		p.nextPort += 2
		if p.nextPort > p.portMax-1 {
			p.nextPort = p.portMin
		}

		if _, taken := p.allocated[port]; taken {
			// If we've wrapped all the way around, no ports available.
			if p.nextPort == startPort {
				return nil, fmt.Errorf("no rtp ports available (all checked)")
			}
			continue
		}

		// Try to bind both RTP and RTCP sockets.
		pair, err := bindPair(port)
		if err != nil {
			p.logger.Debug("port pair bind failed, trying next",
				"rtp_port", port,
				"error", err,
			)
			// Port might be in use by another process; skip it.
			if p.nextPort == startPort {
				return nil, fmt.Errorf("no bindable rtp ports available")
			}
			continue
		}

		p.allocated[port] = struct{}{}

		p.logger.Debug("port pair allocated",
			"rtp_port", port,
			"rtcp_port", port+1,
			"allocated", len(p.allocated),
			"capacity", capacity,
		)

		return pair, nil
	}
}

// Release closes the UDP sockets and returns the port pair to the pool.
func (p *Proxy) Release(pair *SocketPair) {
	if pair == nil {
		return
	}

	if err := pair.Close(); err != nil {
		p.logger.Warn("error closing socket pair",
			"rtp_port", pair.Ports.RTP,
			"error", err,
		)
	}

	p.mu.Lock()
	delete(p.allocated, pair.Ports.RTP)
	count := len(p.allocated)
	p.mu.Unlock()

	p.logger.Debug("port pair released",
		"rtp_port", pair.Ports.RTP,
		"rtcp_port", pair.Ports.RTCP,
		"allocated", count,
	)
}

// bindPair creates UDP sockets bound to the given even port (RTP) and
// its companion odd port (RTCP). If either bind fails, both are cleaned up.
func bindPair(rtpPort int) (*SocketPair, error) {
	rtpAddr := &net.UDPAddr{IP: net.IPv4zero, Port: rtpPort}
	rtpConn, err := net.ListenUDP("udp", rtpAddr)
	if err != nil {
		return nil, fmt.Errorf("binding rtp port %d: %w", rtpPort, err)
	}

	rtcpPort := rtpPort + 1
	rtcpAddr := &net.UDPAddr{IP: net.IPv4zero, Port: rtcpPort}
	rtcpConn, err := net.ListenUDP("udp", rtcpAddr)
	if err != nil {
		rtpConn.Close()
		return nil, fmt.Errorf("binding rtcp port %d: %w", rtcpPort, err)
	}

	return &SocketPair{
		Ports:    PortPair{RTP: rtpPort, RTCP: rtcpPort},
		RTPConn:  rtpConn,
		RTCPConn: rtcpConn,
	}, nil
}
