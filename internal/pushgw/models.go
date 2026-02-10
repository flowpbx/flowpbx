package pushgw

import "time"

// License represents a license key record.
type License struct {
	ID            int64
	Key           string
	Tier          string // "free", "standard", "professional"
	MaxExtensions int
	ExpiresAt     *time.Time
	CreatedAt     time.Time
}

// Installation represents an activated PBX instance for a license.
type Installation struct {
	ID          int64
	LicenseID   int64
	InstanceID  string
	Hostname    string
	Version     string
	ActivatedAt time.Time
	LastSeenAt  time.Time
}

// PushLogEntry represents a single push delivery attempt log record.
type PushLogEntry struct {
	LicenseKey string
	Platform   string
	CallID     string
	Success    bool
	Error      string
	Timestamp  time.Time
}

// PushPayload is the data sent inside a push notification.
type PushPayload struct {
	CallID   string `json:"call_id"`
	CallerID string `json:"caller_id"`
	Type     string `json:"type"` // "incoming_call"
}

// PushRequest is the JSON body for POST /v1/push.
type PushRequest struct {
	LicenseKey   string `json:"license_key"`
	PushToken    string `json:"push_token"`
	PushPlatform string `json:"push_platform"` // "fcm" or "apns"
	CallerID     string `json:"caller_id"`
	CallID       string `json:"call_id"`
}

// PushResponse is the JSON response for POST /v1/push.
type PushResponse struct {
	Delivered bool   `json:"delivered"`
	CallID    string `json:"call_id"`
}

// LicenseValidateRequest is the JSON body for POST /v1/license/validate.
type LicenseValidateRequest struct {
	LicenseKey string `json:"license_key"`
}

// LicenseValidateResponse is the JSON response for POST /v1/license/validate.
type LicenseValidateResponse struct {
	Valid         bool       `json:"valid"`
	Tier          string     `json:"tier"`
	MaxExtensions int        `json:"max_extensions"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

// LicenseActivateRequest is the JSON body for POST /v1/license/activate.
type LicenseActivateRequest struct {
	LicenseKey string `json:"license_key"`
	Hostname   string `json:"hostname"`
	Version    string `json:"version"`
}

// LicenseActivateResponse is the JSON response for POST /v1/license/activate.
type LicenseActivateResponse struct {
	InstanceID  string    `json:"instance_id"`
	ActivatedAt time.Time `json:"activated_at"`
}

// LicenseStatus is the JSON response for GET /v1/license/status.
type LicenseStatus struct {
	Key               string     `json:"key"`
	Tier              string     `json:"tier"`
	MaxExtensions     int        `json:"max_extensions"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	InstallationCount int        `json:"installation_count"`
	Active            bool       `json:"active"`
}
