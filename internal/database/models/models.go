package models

import "time"

// SystemConfig represents a key-value configuration entry.
type SystemConfig struct {
	ID        int64
	Key       string
	Value     string
	UpdatedAt time.Time
}

// Extension represents a PBX extension/user.
type Extension struct {
	ID               int64
	Extension        string
	Name             string
	Email            string
	SIPUsername      string
	SIPPassword      string // hashed
	RingTimeout      int
	DND              bool
	FollowMeEnabled  bool
	FollowMeNumbers  string // JSON
	RecordingMode    string
	MaxRegistrations int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Trunk represents a SIP trunk configuration.
type Trunk struct {
	ID             int64
	Name           string
	Type           string // "register" | "ip"
	Enabled        bool
	Host           string
	Port           int
	Transport      string
	Username       string
	Password       string // encrypted at rest
	AuthUsername   string
	RegisterExpiry int
	RemoteHosts    string // JSON
	LocalHost      string
	Codecs         string // JSON
	MaxChannels    int
	CallerIDName   string
	CallerIDNum    string
	PrefixStrip    int
	PrefixAdd      string
	Priority       int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// InboundNumber represents a DID/inbound number mapping.
type InboundNumber struct {
	ID            int64
	Number        string
	Name          string
	TrunkID       *int64
	FlowID        *int64
	FlowEntryNode string
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// VoicemailBox represents a voicemail box configuration.
type VoicemailBox struct {
	ID                 int64
	Name               string
	MailboxNumber      string
	PIN                string // hashed
	GreetingFile       string
	GreetingType       string
	EmailNotify        bool
	EmailAddress       string
	EmailAttachAudio   bool
	MaxMessageDuration int
	MaxMessages        int
	RetentionDays      int
	NotifyExtensionID  *int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// VoicemailMessage represents a single voicemail message.
type VoicemailMessage struct {
	ID            int64
	MailboxID     int64
	CallerIDName  string
	CallerIDNum   string
	Timestamp     time.Time
	Duration      int
	FilePath      string
	Read          bool
	ReadAt        *time.Time
	Transcription string
	CreatedAt     time.Time
}

// RingGroup represents a ring group configuration.
type RingGroup struct {
	ID           int64
	Name         string
	Strategy     string
	RingTimeout  int
	Members      string // JSON array of extension IDs
	CallerIDMode string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IVRMenu represents an IVR menu configuration.
type IVRMenu struct {
	ID           int64
	Name         string
	GreetingFile string
	GreetingTTS  string
	Timeout      int
	MaxRetries   int
	DigitTimeout int
	Options      string // JSON
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TimeSwitch represents a time-based routing rule set.
type TimeSwitch struct {
	ID          int64
	Name        string
	Timezone    string
	Rules       string // JSON
	DefaultDest string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CallFlow represents a visual call flow graph.
type CallFlow struct {
	ID          int64
	Name        string
	FlowData    string // React Flow JSON
	Version     int
	Published   bool
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CDR represents a call detail record.
type CDR struct {
	ID            int64
	CallID        string
	StartTime     time.Time
	AnswerTime    *time.Time
	EndTime       *time.Time
	Duration      *int
	BillableDur   *int
	CallerIDName  string
	CallerIDNum   string
	Callee        string
	TrunkID       *int64
	Direction     string
	Disposition   string
	RecordingFile string
	FlowPath      string // JSON
	HangupCause   string
}

// Registration represents an active SIP registration.
type Registration struct {
	ID           int64
	ExtensionID  *int64
	ContactURI   string
	Transport    string
	UserAgent    string
	SourceIP     string
	SourcePort   int
	Expires      time.Time
	RegisteredAt time.Time
	PushToken    string
	PushPlatform string
	DeviceID     string
}

// AdminUser represents an admin panel user.
type AdminUser struct {
	ID           int64
	Username     string
	PasswordHash string
	TOTPSecret   *string // nullable, for Phase 2 TOTP 2FA
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ConferenceBridge represents a conference bridge configuration.
type ConferenceBridge struct {
	ID            int64
	Name          string
	Extension     string
	PIN           string
	MaxMembers    int
	Record        bool
	MuteOnJoin    bool
	AnnounceJoins bool
	CreatedAt     time.Time
}
