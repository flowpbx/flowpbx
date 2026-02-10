package media

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// G.711 u-law (PCMU) decoding table: maps each u-law byte to a 16-bit linear PCM sample.
var ulawToLinear [256]int16

// G.711 a-law (PCMA) decoding table: maps each a-law byte to a 16-bit linear PCM sample.
var alawToLinear [256]int16

// G.711 u-law encoding table: maps 14-bit magnitude to a u-law byte.
// Precomputed for the full 16-bit signed range (we use magnitude + sign).
var linearToUlaw [65536]uint8

// G.711 a-law encoding table: maps 16-bit signed sample to an a-law byte.
var linearToAlaw [65536]uint8

func init() {
	// Build u-law decode table.
	for i := 0; i < 256; i++ {
		ulawToLinear[i] = decodeUlaw(uint8(i))
	}
	// Build a-law decode table.
	for i := 0; i < 256; i++ {
		alawToLinear[i] = decodeAlaw(uint8(i))
	}
	// Build u-law encode table.
	for i := -32768; i <= 32767; i++ {
		linearToUlaw[uint16(int16(i))] = encodeUlaw(int16(i))
	}
	// Build a-law encode table.
	for i := -32768; i <= 32767; i++ {
		linearToAlaw[uint16(int16(i))] = encodeAlaw(int16(i))
	}
}

// decodeUlaw converts a u-law byte to a 16-bit linear PCM sample.
func decodeUlaw(u uint8) int16 {
	// Complement to obtain the original code.
	u = ^u
	sign := int16(1)
	if u&0x80 != 0 {
		sign = -1
		u &= 0x7F
	}
	exponent := int((u >> 4) & 0x07)
	mantissa := int(u & 0x0F)
	// Reconstruct the magnitude.
	sample := int16((mantissa<<(uint(exponent)+1) | (1 << uint(exponent)) | (1 << uint(exponent)) - 1 + (1 << uint(exponent))) - 33 + (1 << uint(exponent)))
	// Simplified standard formula:
	sample = int16(((2*mantissa + 33) << uint(exponent)) - 33)
	return sign * sample
}

// decodeAlaw converts an a-law byte to a 16-bit linear PCM sample.
func decodeAlaw(a uint8) int16 {
	a ^= 0x55
	sign := int16(1)
	if a&0x80 != 0 {
		a &= 0x7F
	} else {
		sign = -1
	}
	exponent := int((a >> 4) & 0x07)
	mantissa := int(a & 0x0F)
	var sample int16
	if exponent == 0 {
		sample = int16(mantissa<<4 | 0x08)
	} else {
		sample = int16((mantissa<<4 | 0x108) << uint(exponent-1))
	}
	return sign * sample
}

// encodeUlaw converts a 16-bit linear PCM sample to a u-law byte.
func encodeUlaw(sample int16) uint8 {
	// Bias and clamp.
	const bias = 0x84
	const clip = 32635

	sign := uint8(0)
	if sample < 0 {
		sign = 0x80
		sample = -sample
	}
	if sample > clip {
		sample = clip
	}
	sample += bias

	exponent := 7
	mask := int16(0x4000)
	for exponent > 0 {
		if sample&mask != 0 {
			break
		}
		exponent--
		mask >>= 1
	}

	mantissa := (sample >> (uint(exponent) + 3)) & 0x0F
	uval := ^(sign | uint8(exponent<<4) | uint8(mantissa))
	return uval
}

// encodeAlaw converts a 16-bit linear PCM sample to an a-law byte.
func encodeAlaw(sample int16) uint8 {
	sign := uint8(0x55)
	if sample < 0 {
		sample = -sample
		sign = 0xD5
	}

	if sample > 4095 {
		sample = 4095
	}

	var exponent int
	var mantissa int
	if sample < 256 {
		exponent = 0
		mantissa = int(sample) >> 4
	} else {
		exp := 1
		expMask := int16(512)
		for exp < 7 {
			if sample < expMask<<1 {
				break
			}
			exp++
			expMask <<= 1
		}
		exponent = exp
		mantissa = (int(sample) >> uint(exponent+3)) & 0x0F
	}

	aval := uint8(exponent<<4 | mantissa)
	return aval ^ sign
}

// MixerParticipant represents a single participant in a conference mix.
type MixerParticipant struct {
	// ID uniquely identifies this participant (typically the call ID).
	ID string

	// Muted indicates whether this participant's audio is excluded from the mix.
	// When muted, the participant still receives mixed audio from others.
	muted atomic.Bool

	// Socket is the RTP socket pair allocated for this participant's leg.
	Socket *SocketPair

	// Remote is the learned remote RTP address for this participant.
	remote *atomicAddr

	// payloadType is the negotiated audio codec (PayloadPCMU or PayloadPCMA).
	payloadType int

	// ssrc is the RTP SSRC for outbound packets to this participant.
	ssrc uint32

	// seq is the next RTP sequence number for outbound packets.
	seq uint16

	// ts is the next RTP timestamp for outbound packets.
	ts uint32

	// lastAudio stores the most recent decoded linear PCM frame from this
	// participant. Protected by the mixer's frame lock.
	lastAudio [samplesPerPacket]int16

	// hasAudio indicates whether lastAudio contains valid data for the
	// current mix cycle.
	hasAudio bool
}

// SetMuted sets the mute state for this participant.
func (p *MixerParticipant) SetMuted(muted bool) {
	p.muted.Store(muted)
}

// IsMuted returns true if this participant is muted.
func (p *MixerParticipant) IsMuted() bool {
	return p.muted.Load()
}

// Mixer implements N-way audio mixing for conference bridges.
//
// Architecture: The mixer allocates one RTP socket pair per participant.
// A single mix goroutine runs at the ptime interval (20ms). On each cycle:
//  1. Read one RTP packet from each participant's socket (non-blocking).
//  2. Decode G.711 audio to linear PCM (16-bit signed).
//  3. For each participant, sum all OTHER participants' decoded audio (N-1 mix).
//  4. Encode the mixed PCM back to the participant's G.711 codec.
//  5. Send the mixed RTP packet to the participant.
//
// This "decode, mix, encode" approach ensures each participant hears all
// other participants mixed together, but not their own audio (avoiding echo).
type Mixer struct {
	proxy  *Proxy
	logger *slog.Logger

	mu           sync.RWMutex
	participants map[string]*MixerParticipant
	stopped      atomic.Bool
	mixDone      chan struct{}

	// toneMu guards toneFrames and tonePos. When toneFrames is non-nil,
	// the mix loop adds the tone audio to every participant's output.
	toneMu     sync.Mutex
	toneFrames []int16 // pre-generated linear PCM tone samples
	tonePos    int     // current read position in toneFrames
}

// NewMixer creates a new conference audio mixer backed by the given proxy
// for port allocation.
func NewMixer(proxy *Proxy, logger *slog.Logger) *Mixer {
	return &Mixer{
		proxy:        proxy,
		logger:       logger.With("subsystem", "conference-mixer"),
		participants: make(map[string]*MixerParticipant),
	}
}

// AddParticipant allocates an RTP socket pair for a new participant and
// registers them in the mixer. The remote address is the participant's
// far-end RTP address from SDP negotiation. payloadType must be PayloadPCMU
// or PayloadPCMA.
//
// Returns the allocated SocketPair (for SDP rewriting) and an error if
// allocation fails.
func (m *Mixer) AddParticipant(id string, remote *net.UDPAddr, payloadType int) (*SocketPair, error) {
	if payloadType != PayloadPCMU && payloadType != PayloadPCMA {
		return nil, fmt.Errorf("unsupported conference codec: payload type %d, only PCMU (0) and PCMA (8) supported", payloadType)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.participants[id]; exists {
		return nil, fmt.Errorf("participant %q already in conference", id)
	}

	pair, err := m.proxy.Allocate()
	if err != nil {
		return nil, fmt.Errorf("allocating conference port for %q: %w", id, err)
	}

	p := &MixerParticipant{
		ID:          id,
		Socket:      pair,
		remote:      newAtomicAddr(remote),
		payloadType: payloadType,
		ssrc:        rand.Uint32(),
		seq:         uint16(rand.UintN(65536)),
		ts:          rand.Uint32(),
	}

	m.participants[id] = p

	m.logger.Info("participant added to conference",
		"participant_id", id,
		"rtp_port", pair.Ports.RTP,
		"remote", remote.String(),
		"payload_type", payloadType,
		"total_participants", len(m.participants),
	)

	return pair, nil
}

// RemoveParticipant removes a participant from the mixer and releases their
// RTP port pair. Returns an error if the participant is not found.
func (m *Mixer) RemoveParticipant(id string) error {
	m.mu.Lock()
	p, exists := m.participants[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("participant %q not in conference", id)
	}
	delete(m.participants, id)
	count := len(m.participants)
	m.mu.Unlock()

	m.proxy.Release(p.Socket)

	m.logger.Info("participant removed from conference",
		"participant_id", id,
		"remaining_participants", count,
	)

	return nil
}

// ParticipantCount returns the number of active participants.
func (m *Mixer) ParticipantCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.participants)
}

// GetParticipant returns the participant with the given ID, or nil if not found.
func (m *Mixer) GetParticipant(id string) *MixerParticipant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.participants[id]
}

// ParticipantIDs returns a snapshot of all current participant IDs.
func (m *Mixer) ParticipantIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.participants))
	for id := range m.participants {
		ids = append(ids, id)
	}
	return ids
}

// Start begins the conference mix loop. It runs in a background goroutine
// until Stop is called. The context is used for cancellation.
func (m *Mixer) Start(ctx context.Context) {
	m.mixDone = make(chan struct{})
	go m.mixLoop(ctx)

	m.logger.Info("conference mixer started")
}

// Stop signals the mix loop to stop and waits for it to finish.
func (m *Mixer) Stop() {
	m.stopped.Store(true)
	if m.mixDone != nil {
		<-m.mixDone
	}
	m.logger.Info("conference mixer stopped")
}

// Release stops the mixer and releases all participant port pairs.
func (m *Mixer) Release() {
	m.Stop()

	m.mu.Lock()
	participants := make([]*MixerParticipant, 0, len(m.participants))
	for _, p := range m.participants {
		participants = append(participants, p)
	}
	m.participants = make(map[string]*MixerParticipant)
	m.mu.Unlock()

	for _, p := range participants {
		m.proxy.Release(p.Socket)
	}

	m.logger.Info("conference mixer released",
		"participants_released", len(participants),
	)
}

// mixLoop is the core conference mixing goroutine. It runs every 20ms
// (one ptime interval), reads audio from all participants, mixes, and
// sends the result back.
func (m *Mixer) mixLoop(ctx context.Context) {
	defer close(m.mixDone)

	ticker := time.NewTicker(packetDuration)
	defer ticker.Stop()

	buf := make([]byte, maxRTPPacket)
	outPkt := make([]byte, rtpHeaderSize+samplesPerPacket)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if m.stopped.Load() {
				return
			}
			m.mixCycle(buf, outPkt)
		}
	}
}

// mixCycle performs one iteration of the mix loop:
// 1. Read and decode audio from each participant.
// 2. Compute N-1 mix for each participant.
// 3. Encode and send mixed audio to each participant.
func (m *Mixer) mixCycle(readBuf, outPkt []byte) {
	m.mu.RLock()
	count := len(m.participants)
	if count == 0 {
		m.mu.RUnlock()
		return
	}

	// Collect participants into a slice for stable iteration.
	parts := make([]*MixerParticipant, 0, count)
	for _, p := range m.participants {
		parts = append(parts, p)
	}
	m.mu.RUnlock()

	// Phase 1: Read one RTP packet from each participant and decode to PCM.
	for _, p := range parts {
		p.hasAudio = false

		// Non-blocking read with short deadline.
		p.Socket.RTPConn.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
		n, srcAddr, err := p.Socket.RTPConn.ReadFromUDP(readBuf)
		if err != nil {
			if !errors.Is(err, os.ErrDeadlineExceeded) {
				m.logger.Debug("conference read error",
					"participant_id", p.ID,
					"error", err,
				)
			}
			continue
		}

		pkt := readBuf[:n]

		// Validate minimum RTP size.
		if n < minRTPHeader+1 {
			continue
		}

		pt := rtpPayloadType(pkt)
		if pt != p.payloadType {
			// Skip non-audio packets (DTMF, etc.).
			continue
		}

		// Symmetric RTP: learn actual remote address from first packet.
		p.remote.update(srcAddr)

		// Decode G.711 payload to linear PCM.
		payload := pkt[minRTPHeader:]
		samples := len(payload)
		if samples > samplesPerPacket {
			samples = samplesPerPacket
		}

		if p.IsMuted() {
			// Muted participants don't contribute audio.
			continue
		}

		switch pt {
		case PayloadPCMU:
			for i := 0; i < samples; i++ {
				p.lastAudio[i] = ulawToLinear[payload[i]]
			}
		case PayloadPCMA:
			for i := 0; i < samples; i++ {
				p.lastAudio[i] = alawToLinear[payload[i]]
			}
		}
		// Zero-fill remaining samples if packet was short.
		for i := samples; i < samplesPerPacket; i++ {
			p.lastAudio[i] = 0
		}
		p.hasAudio = true
	}

	// Drain any active tone samples for this cycle. The tone is added to
	// every participant's output so all hear the join/leave notification.
	var toneBuf [samplesPerPacket]int16
	hasTone := m.drainTone(toneBuf[:], samplesPerPacket) > 0

	// Phase 2: For each participant, compute the N-1 mix (sum of all others)
	// and send the mixed packet.
	var mixBuf [samplesPerPacket]int32

	for _, dest := range parts {
		// Sum all other participants' audio.
		for i := range mixBuf {
			mixBuf[i] = 0
		}

		hasInput := false
		for _, src := range parts {
			if src.ID == dest.ID {
				continue
			}
			if !src.hasAudio {
				continue
			}
			hasInput = true
			for i := 0; i < samplesPerPacket; i++ {
				mixBuf[i] += int32(src.lastAudio[i])
			}
		}

		// Mix in the tone if active.
		if hasTone {
			hasInput = true
			for i := 0; i < samplesPerPacket; i++ {
				mixBuf[i] += int32(toneBuf[i])
			}
		}

		if !hasInput {
			// No audio from anyone else; send silence or skip.
			// Advance sequence/timestamp to maintain timing.
			dest.seq++
			dest.ts += timestampIncrement
			continue
		}

		// Clamp to 16-bit range and encode to the destination's codec.
		switch dest.payloadType {
		case PayloadPCMU:
			for i := 0; i < samplesPerPacket; i++ {
				s := mixBuf[i]
				if s > 32767 {
					s = 32767
				} else if s < -32768 {
					s = -32768
				}
				outPkt[rtpHeaderSize+i] = linearToUlaw[uint16(int16(s))]
			}
		case PayloadPCMA:
			for i := 0; i < samplesPerPacket; i++ {
				s := mixBuf[i]
				if s > 32767 {
					s = 32767
				} else if s < -32768 {
					s = -32768
				}
				outPkt[rtpHeaderSize+i] = linearToAlaw[uint16(int16(s))]
			}
		}

		// Build RTP header.
		buildRTPHeader(outPkt[:rtpHeaderSize], dest.payloadType, false, dest.seq, dest.ts, dest.ssrc)

		// Send to participant.
		remote := dest.remote.load()
		if remote != nil {
			if _, err := dest.Socket.RTPConn.WriteToUDP(outPkt[:rtpHeaderSize+samplesPerPacket], remote); err != nil {
				m.logger.Debug("conference write error",
					"participant_id", dest.ID,
					"error", err,
				)
			}
		}

		dest.seq++
		dest.ts += timestampIncrement
	}
}

// PortForParticipant returns the local RTP port allocated for the given
// participant. Returns 0 if the participant is not found.
func (m *Mixer) PortForParticipant(id string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.participants[id]
	if !ok {
		return 0
	}
	return p.Socket.Ports.RTP
}

// MixerStats holds statistics for the conference mixer.
type MixerStats struct {
	ParticipantCount int
	ParticipantIDs   []string
}

// Stats returns a snapshot of the mixer's current state.
func (m *Mixer) Stats() MixerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.participants))
	for id := range m.participants {
		ids = append(ids, id)
	}
	return MixerStats{
		ParticipantCount: len(m.participants),
		ParticipantIDs:   ids,
	}
}

// InjectTone pre-generates a short beep and queues it to be mixed into
// all participants' output during the next mix cycles. The tone is a
// 440 Hz sine wave at the given amplitude (0.0–1.0) lasting durationMs.
// It is safe to call from any goroutine; the mix loop drains the tone
// buffer automatically.
func (m *Mixer) InjectTone(frequencyHz float64, amplitude float64, durationMs int) {
	samples := generateBeep(frequencyHz, amplitude, durationMs)

	m.toneMu.Lock()
	m.toneFrames = samples
	m.tonePos = 0
	m.toneMu.Unlock()

	m.logger.Debug("tone injected into conference",
		"frequency_hz", frequencyHz,
		"duration_ms", durationMs,
	)
}

// generateBeep creates linear PCM samples for a sine-wave tone at the
// given frequency, amplitude (0.0–1.0 of int16 range), and duration.
// Sample rate is 8000 Hz to match the G.711 conference clock.
func generateBeep(frequencyHz float64, amplitude float64, durationMs int) []int16 {
	const sampleRate = 8000
	totalSamples := sampleRate * durationMs / 1000
	samples := make([]int16, totalSamples)
	peak := amplitude * 32767.0

	for i := 0; i < totalSamples; i++ {
		t := float64(i) / float64(sampleRate)
		samples[i] = int16(peak * math.Sin(2.0*math.Pi*frequencyHz*t))
	}

	return samples
}

// drainTone copies up to n samples from the active tone buffer into dst,
// returning the number of samples written. Advances the read position.
// When the tone is fully drained, the buffer is cleared.
func (m *Mixer) drainTone(dst []int16, n int) int {
	m.toneMu.Lock()
	defer m.toneMu.Unlock()

	if m.toneFrames == nil {
		return 0
	}

	remaining := len(m.toneFrames) - m.tonePos
	if remaining <= 0 {
		m.toneFrames = nil
		m.tonePos = 0
		return 0
	}

	count := n
	if count > remaining {
		count = remaining
	}
	copy(dst[:count], m.toneFrames[m.tonePos:m.tonePos+count])
	m.tonePos += count

	// Clear when fully consumed.
	if m.tonePos >= len(m.toneFrames) {
		m.toneFrames = nil
		m.tonePos = 0
	}

	return count
}

// RTPTimestamp extracts the 32-bit RTP timestamp from a packet header.
func RTPTimestamp(pkt []byte) uint32 {
	if len(pkt) < minRTPHeader {
		return 0
	}
	return binary.BigEndian.Uint32(pkt[4:8])
}
