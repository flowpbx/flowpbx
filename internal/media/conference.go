package media

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

// ConferenceRoom represents an active conference room backed by an audio Mixer.
// It is created on-demand when the first participant joins and destroyed when
// the last participant leaves.
type ConferenceRoom struct {
	BridgeID      int64
	BridgeName    string
	Mixer         *Mixer
	MaxMembers    int
	AnnounceJoins bool

	// done is closed when the room is empty and should be removed.
	done chan struct{}
}

// ConferenceParticipant holds information about a participant in a conference room.
type ConferenceParticipant struct {
	ID          string // call ID
	BridgeID    int64
	PayloadType int
	Port        int // local RTP port allocated for this participant
	Muted       bool
}

// ConferenceManager manages active conference rooms, mapping bridge IDs to
// live Mixer instances. It handles the full lifecycle: create room on first
// join, add/remove participants, kick, and destroy room when empty.
type ConferenceManager struct {
	proxy  *Proxy
	logger *slog.Logger

	mu    sync.Mutex
	rooms map[int64]*ConferenceRoom
}

// NewConferenceManager creates a conference manager backed by the given proxy
// for RTP port allocation.
func NewConferenceManager(proxy *Proxy, logger *slog.Logger) *ConferenceManager {
	return &ConferenceManager{
		proxy:  proxy,
		logger: logger.With("subsystem", "conference-manager"),
		rooms:  make(map[int64]*ConferenceRoom),
	}
}

// JoinResult holds the result of joining a conference.
type JoinResult struct {
	// Room is the conference room that was joined.
	Room *ConferenceRoom
	// Socket is the RTP socket pair allocated for this participant.
	Socket *SocketPair
	// Port is the local RTP port for SDP rewriting.
	Port int
}

// conferenceJoinToneHz is the frequency (Hz) of the tone played when a
// participant joins or leaves a conference with announce_joins enabled.
const conferenceJoinToneHz = 440.0

// conferenceJoinToneAmplitude is the amplitude (0.0–1.0) of the join/leave tone.
const conferenceJoinToneAmplitude = 0.25

// conferenceJoinToneDurationMs is the duration in milliseconds of the join tone.
const conferenceJoinToneDurationMs = 200

// conferenceLeaveToneDurationMs is the duration in milliseconds of the leave tone.
// A shorter tone distinguishes leave from join.
const conferenceLeaveToneDurationMs = 100

// Join adds a participant to a conference room. If the room does not exist,
// it is created and the mixer is started. Returns the allocated RTP socket pair
// for SDP rewriting.
//
// The caller must call Leave when the participant exits the conference.
func (cm *ConferenceManager) Join(ctx context.Context, bridgeID int64, bridgeName string, maxMembers int, announceJoins bool, participantID string, remote *net.UDPAddr, payloadType int) (*JoinResult, error) {
	cm.mu.Lock()

	room, exists := cm.rooms[bridgeID]
	if !exists {
		mixer := NewMixer(cm.proxy, cm.logger)
		room = &ConferenceRoom{
			BridgeID:      bridgeID,
			BridgeName:    bridgeName,
			Mixer:         mixer,
			MaxMembers:    maxMembers,
			AnnounceJoins: announceJoins,
			done:          make(chan struct{}),
		}
		cm.rooms[bridgeID] = room
		mixer.Start(ctx)

		cm.logger.Info("conference room created",
			"bridge_id", bridgeID,
			"bridge_name", bridgeName,
			"max_members", maxMembers,
			"announce_joins", announceJoins,
		)
	}

	// Check max_members limit before adding.
	current := room.Mixer.ParticipantCount()
	if maxMembers > 0 && current >= maxMembers {
		cm.mu.Unlock()
		return nil, fmt.Errorf("conference %q is full (%d/%d members)", bridgeName, current, maxMembers)
	}

	cm.mu.Unlock()

	// Add participant to the mixer (mixer has its own locking).
	socket, err := room.Mixer.AddParticipant(participantID, remote, payloadType)
	if err != nil {
		return nil, fmt.Errorf("adding participant to conference %q: %w", bridgeName, err)
	}

	cm.logger.Info("participant joined conference",
		"bridge_id", bridgeID,
		"bridge_name", bridgeName,
		"participant_id", participantID,
		"rtp_port", socket.Ports.RTP,
		"participants", room.Mixer.ParticipantCount(),
	)

	// Play join tone to all participants if announce_joins is enabled.
	if room.AnnounceJoins {
		room.Mixer.InjectTone(conferenceJoinToneHz, conferenceJoinToneAmplitude, conferenceJoinToneDurationMs)
	}

	return &JoinResult{
		Room:   room,
		Socket: socket,
		Port:   socket.Ports.RTP,
	}, nil
}

// Leave removes a participant from a conference room. If the room is empty
// after removal, it is destroyed and its mixer is released.
func (cm *ConferenceManager) Leave(bridgeID int64, participantID string) error {
	cm.mu.Lock()
	room, exists := cm.rooms[bridgeID]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("conference room %d not found", bridgeID)
	}
	cm.mu.Unlock()

	if err := room.Mixer.RemoveParticipant(participantID); err != nil {
		return fmt.Errorf("removing participant from conference: %w", err)
	}

	remaining := room.Mixer.ParticipantCount()

	cm.logger.Info("participant left conference",
		"bridge_id", bridgeID,
		"bridge_name", room.BridgeName,
		"participant_id", participantID,
		"remaining", remaining,
	)

	// Play leave tone to remaining participants if announce_joins is enabled
	// and the room is not empty.
	if room.AnnounceJoins && remaining > 0 {
		room.Mixer.InjectTone(conferenceJoinToneHz, conferenceJoinToneAmplitude, conferenceLeaveToneDurationMs)
	}

	// If room is empty, destroy it.
	cm.mu.Lock()
	if room.Mixer.ParticipantCount() == 0 {
		delete(cm.rooms, bridgeID)
		cm.mu.Unlock()

		room.Mixer.Release()
		close(room.done)

		cm.logger.Info("conference room destroyed (empty)",
			"bridge_id", bridgeID,
			"bridge_name", room.BridgeName,
		)
		return nil
	}
	cm.mu.Unlock()

	return nil
}

// Kick removes a participant from a conference room forcibly. This is the
// same as Leave but uses different logging for audit purposes.
func (cm *ConferenceManager) Kick(bridgeID int64, participantID string) error {
	cm.logger.Info("kicking participant from conference",
		"bridge_id", bridgeID,
		"participant_id", participantID,
	)
	return cm.Leave(bridgeID, participantID)
}

// MuteParticipant sets the mute state for a participant in a conference room.
func (cm *ConferenceManager) MuteParticipant(bridgeID int64, participantID string, muted bool) error {
	cm.mu.Lock()
	room, exists := cm.rooms[bridgeID]
	cm.mu.Unlock()

	if !exists {
		return fmt.Errorf("conference room %d not found", bridgeID)
	}

	p := room.Mixer.GetParticipant(participantID)
	if p == nil {
		return fmt.Errorf("participant %q not in conference %d", participantID, bridgeID)
	}

	p.SetMuted(muted)

	cm.logger.Info("participant mute state changed",
		"bridge_id", bridgeID,
		"participant_id", participantID,
		"muted", muted,
	)

	return nil
}

// GetRoom returns the active conference room for the given bridge ID, or nil.
func (cm *ConferenceManager) GetRoom(bridgeID int64) *ConferenceRoom {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.rooms[bridgeID]
}

// ActiveRooms returns the number of currently active conference rooms.
func (cm *ConferenceManager) ActiveRooms() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.rooms)
}

// Participants returns the list of participants in a conference room.
func (cm *ConferenceManager) Participants(bridgeID int64) ([]ConferenceParticipant, error) {
	cm.mu.Lock()
	room, exists := cm.rooms[bridgeID]
	cm.mu.Unlock()

	if !exists {
		return nil, nil
	}

	ids := room.Mixer.ParticipantIDs()
	result := make([]ConferenceParticipant, 0, len(ids))
	for _, id := range ids {
		p := room.Mixer.GetParticipant(id)
		if p == nil {
			continue
		}
		result = append(result, ConferenceParticipant{
			ID:          id,
			BridgeID:    bridgeID,
			PayloadType: p.payloadType,
			Port:        p.Socket.Ports.RTP,
			Muted:       p.IsMuted(),
		})
	}

	return result, nil
}

// ReleaseAll stops and releases all active conference rooms. Used during shutdown.
func (cm *ConferenceManager) ReleaseAll() {
	cm.mu.Lock()
	rooms := make([]*ConferenceRoom, 0, len(cm.rooms))
	for _, room := range cm.rooms {
		rooms = append(rooms, room)
	}
	cm.rooms = make(map[int64]*ConferenceRoom)
	cm.mu.Unlock()

	for _, room := range rooms {
		room.Mixer.Release()
		close(room.done)
	}

	cm.logger.Info("all conference rooms released", "count", len(rooms))
}
