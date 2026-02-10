package pushgw

import "fmt"

// MultiSender routes push notifications to the appropriate platform sender.
type MultiSender struct {
	senders map[string]PushSender
}

// NewMultiSender creates a MultiSender from a map of platform name to sender.
// At least one sender must be provided.
func NewMultiSender(senders map[string]PushSender) *MultiSender {
	return &MultiSender{senders: senders}
}

// Send delegates to the sender registered for the given platform.
func (m *MultiSender) Send(platform, token string, payload PushPayload) error {
	s, ok := m.senders[platform]
	if !ok {
		return fmt.Errorf("no sender configured for platform %q", platform)
	}
	return s.Send(platform, token, payload)
}
