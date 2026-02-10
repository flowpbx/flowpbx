package database

import (
	"context"

	"github.com/flowpbx/flowpbx/internal/database/models"
)

// SystemConfigRepository manages key-value system configuration.
type SystemConfigRepository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetAll(ctx context.Context) ([]models.SystemConfig, error)
}

// AdminUserRepository manages admin panel users.
type AdminUserRepository interface {
	Create(ctx context.Context, user *models.AdminUser) error
	GetByID(ctx context.Context, id int64) (*models.AdminUser, error)
	GetByUsername(ctx context.Context, username string) (*models.AdminUser, error)
	List(ctx context.Context) ([]models.AdminUser, error)
	Update(ctx context.Context, user *models.AdminUser) error
	Delete(ctx context.Context, id int64) error
	Count(ctx context.Context) (int64, error)
}

// ExtensionRepository manages PBX extensions/users.
type ExtensionRepository interface {
	Create(ctx context.Context, ext *models.Extension) error
	GetByID(ctx context.Context, id int64) (*models.Extension, error)
	GetByExtension(ctx context.Context, ext string) (*models.Extension, error)
	GetBySIPUsername(ctx context.Context, username string) (*models.Extension, error)
	List(ctx context.Context) ([]models.Extension, error)
	Update(ctx context.Context, ext *models.Extension) error
	Delete(ctx context.Context, id int64) error
}

// TrunkRepository manages SIP trunks.
type TrunkRepository interface {
	Create(ctx context.Context, trunk *models.Trunk) error
	GetByID(ctx context.Context, id int64) (*models.Trunk, error)
	List(ctx context.Context) ([]models.Trunk, error)
	ListEnabled(ctx context.Context) ([]models.Trunk, error)
	Update(ctx context.Context, trunk *models.Trunk) error
	Delete(ctx context.Context, id int64) error
}

// InboundNumberRepository manages DID/inbound number mappings.
type InboundNumberRepository interface {
	Create(ctx context.Context, num *models.InboundNumber) error
	GetByID(ctx context.Context, id int64) (*models.InboundNumber, error)
	GetByNumber(ctx context.Context, number string) (*models.InboundNumber, error)
	List(ctx context.Context) ([]models.InboundNumber, error)
	Update(ctx context.Context, num *models.InboundNumber) error
	Delete(ctx context.Context, id int64) error
}

// VoicemailBoxRepository manages voicemail box configurations.
type VoicemailBoxRepository interface {
	Create(ctx context.Context, box *models.VoicemailBox) error
	GetByID(ctx context.Context, id int64) (*models.VoicemailBox, error)
	List(ctx context.Context) ([]models.VoicemailBox, error)
	Update(ctx context.Context, box *models.VoicemailBox) error
	Delete(ctx context.Context, id int64) error
}

// VoicemailMessageRepository manages voicemail messages.
type VoicemailMessageRepository interface {
	Create(ctx context.Context, msg *models.VoicemailMessage) error
	GetByID(ctx context.Context, id int64) (*models.VoicemailMessage, error)
	ListByMailbox(ctx context.Context, mailboxID int64) ([]models.VoicemailMessage, error)
	MarkRead(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
	DeleteExpiredByRetention(ctx context.Context) ([]string, error)
	CountByMailbox(ctx context.Context, mailboxID int64) (int64, error)
}

// RingGroupRepository manages ring groups.
type RingGroupRepository interface {
	Create(ctx context.Context, rg *models.RingGroup) error
	GetByID(ctx context.Context, id int64) (*models.RingGroup, error)
	List(ctx context.Context) ([]models.RingGroup, error)
	Update(ctx context.Context, rg *models.RingGroup) error
	Delete(ctx context.Context, id int64) error
}

// IVRMenuRepository manages IVR menus.
type IVRMenuRepository interface {
	Create(ctx context.Context, ivr *models.IVRMenu) error
	GetByID(ctx context.Context, id int64) (*models.IVRMenu, error)
	List(ctx context.Context) ([]models.IVRMenu, error)
	Update(ctx context.Context, ivr *models.IVRMenu) error
	Delete(ctx context.Context, id int64) error
}

// TimeSwitchRepository manages time switch rules.
type TimeSwitchRepository interface {
	Create(ctx context.Context, ts *models.TimeSwitch) error
	GetByID(ctx context.Context, id int64) (*models.TimeSwitch, error)
	List(ctx context.Context) ([]models.TimeSwitch, error)
	Update(ctx context.Context, ts *models.TimeSwitch) error
	Delete(ctx context.Context, id int64) error
}

// CallFlowRepository manages call flow graphs.
type CallFlowRepository interface {
	Create(ctx context.Context, flow *models.CallFlow) error
	GetByID(ctx context.Context, id int64) (*models.CallFlow, error)
	GetPublished(ctx context.Context, id int64) (*models.CallFlow, error)
	List(ctx context.Context) ([]models.CallFlow, error)
	Update(ctx context.Context, flow *models.CallFlow) error
	Publish(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
}

// CDRListFilter specifies filtering and pagination for CDR list queries.
type CDRListFilter struct {
	Limit     int
	Offset    int
	Search    string // matches caller_id_name, caller_id_num, or callee
	Direction string // "inbound", "outbound", "internal", or "" for all
	StartDate string // RFC3339 or YYYY-MM-DD
	EndDate   string // RFC3339 or YYYY-MM-DD
}

// CDRRepository manages call detail records.
type CDRRepository interface {
	Create(ctx context.Context, cdr *models.CDR) error
	GetByID(ctx context.Context, id int64) (*models.CDR, error)
	GetByCallID(ctx context.Context, callID string) (*models.CDR, error)
	Update(ctx context.Context, cdr *models.CDR) error
	List(ctx context.Context, filter CDRListFilter) ([]models.CDR, int, error)
	ListRecent(ctx context.Context, limit int) ([]models.CDR, error)
	ListWithRecordings(ctx context.Context, filter CDRListFilter) ([]models.CDR, int, error)
}

// RegistrationRepository manages active SIP registrations.
type RegistrationRepository interface {
	Create(ctx context.Context, reg *models.Registration) error
	GetByExtensionID(ctx context.Context, extensionID int64) ([]models.Registration, error)
	DeleteByID(ctx context.Context, id int64) error
	DeleteExpired(ctx context.Context) (int64, error)
	DeleteAll(ctx context.Context) (int64, error)
	DeleteByExtensionAndContact(ctx context.Context, extensionID int64, contactURI string) error
	CountByExtensionID(ctx context.Context, extensionID int64) (int64, error)
	Count(ctx context.Context) (int64, error)
}

// AudioPromptRepository manages custom audio prompts.
type AudioPromptRepository interface {
	Create(ctx context.Context, prompt *models.AudioPrompt) error
	GetByID(ctx context.Context, id int64) (*models.AudioPrompt, error)
	List(ctx context.Context) ([]models.AudioPrompt, error)
	Delete(ctx context.Context, id int64) error
}

// ConferenceBridgeRepository manages conference bridges.
type ConferenceBridgeRepository interface {
	Create(ctx context.Context, bridge *models.ConferenceBridge) error
	GetByID(ctx context.Context, id int64) (*models.ConferenceBridge, error)
	GetByExtension(ctx context.Context, ext string) (*models.ConferenceBridge, error)
	List(ctx context.Context) ([]models.ConferenceBridge, error)
	Update(ctx context.Context, bridge *models.ConferenceBridge) error
	Delete(ctx context.Context, id int64) error
}
