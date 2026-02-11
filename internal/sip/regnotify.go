package sip

import (
	"context"
	"sync"
)

// RegistrationNotifier provides a pub/sub mechanism for SIP registration events.
// When a mobile app wakes up from a push notification and sends a REGISTER, the
// registrar publishes an event. Push-wait callers can subscribe to be notified
// when a specific extension registers, allowing them to retry ringing the
// extension rather than immediately failing the call.
type RegistrationNotifier struct {
	mu        sync.Mutex
	listeners map[int64][]chan struct{}
}

// NewRegistrationNotifier creates a new RegistrationNotifier.
func NewRegistrationNotifier() *RegistrationNotifier {
	return &RegistrationNotifier{
		listeners: make(map[int64][]chan struct{}),
	}
}

// Subscribe returns a channel that will be closed when the given extension
// registers. The caller should call the returned cancel function to clean up
// when done waiting (whether due to registration or timeout).
func (n *RegistrationNotifier) Subscribe(extensionID int64) (<-chan struct{}, func()) {
	ch := make(chan struct{})

	n.mu.Lock()
	n.listeners[extensionID] = append(n.listeners[extensionID], ch)
	n.mu.Unlock()

	cancel := func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		chs := n.listeners[extensionID]
		for i, c := range chs {
			if c == ch {
				n.listeners[extensionID] = append(chs[:i], chs[i+1:]...)
				break
			}
		}
		if len(n.listeners[extensionID]) == 0 {
			delete(n.listeners, extensionID)
		}
	}

	return ch, cancel
}

// Notify signals all subscribers waiting for the given extension to register.
// This is called by the Registrar after a successful REGISTER.
func (n *RegistrationNotifier) Notify(extensionID int64) {
	n.mu.Lock()
	chs := n.listeners[extensionID]
	// Remove all listeners for this extension — each push-wait is one-shot.
	delete(n.listeners, extensionID)
	n.mu.Unlock()

	for _, ch := range chs {
		close(ch)
	}
}

// WaitForRegistration blocks until either the extension registers or the
// context is cancelled (timeout). Returns true if a registration was received.
func (n *RegistrationNotifier) WaitForRegistration(ctx context.Context, extensionID int64) bool {
	ch, cancel := n.Subscribe(extensionID)
	defer cancel()

	select {
	case <-ch:
		return true
	case <-ctx.Done():
		return false
	}
}
