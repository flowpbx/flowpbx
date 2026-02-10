package media

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Default timing constants for DTMF digit collection.
const (
	// DefaultFirstDigitTimeout is the maximum time to wait for the first
	// digit before declaring a timeout. Typically 5-10 seconds for IVR menus.
	DefaultFirstDigitTimeout = 5 * time.Second

	// DefaultInterDigitTimeout is the maximum time to wait between
	// consecutive digits before delivering the collected input. Standard
	// PBX inter-digit timeout is 3 seconds.
	DefaultInterDigitTimeout = 3 * time.Second
)

// DigitBufferResult holds the outcome of a digit collection operation.
type DigitBufferResult struct {
	// Digits is the collected string of DTMF digits.
	Digits string

	// TimedOut is true if no digit was received before the first-digit
	// timeout, or if the inter-digit timeout expired before max digits
	// were reached. When TimedOut is true and Digits is non-empty, the
	// inter-digit timeout fired (partial input delivered).
	TimedOut bool
}

// DigitBuffer accumulates DTMF digits from a source channel and applies
// inter-digit timeout logic for multi-digit input collection. It reads
// digits from the Digits channel of a DTMFCollector (or any chan string)
// and waits for collection to complete based on timing constraints.
//
// The buffer supports two timeout phases:
//  1. First-digit timeout: how long to wait for the very first digit.
//  2. Inter-digit timeout: how long to wait between consecutive digits
//     before delivering the accumulated input.
//
// Collection ends when any of these conditions is met:
//   - The inter-digit timeout expires after receiving at least one digit
//   - The first-digit timeout expires with no input
//   - The context is cancelled
type DigitBuffer struct {
	source            <-chan string
	firstDigitTimeout time.Duration
	interDigitTimeout time.Duration
	logger            *slog.Logger

	mu     sync.Mutex
	digits []byte // accumulated digits
	lastAt time.Time
}

// NewDigitBuffer creates a buffer that reads from the given digit source
// channel and applies inter-digit timeout logic. The source is typically
// DTMFCollector.Digits.
func NewDigitBuffer(source <-chan string, logger *slog.Logger) *DigitBuffer {
	return &DigitBuffer{
		source:            source,
		firstDigitTimeout: DefaultFirstDigitTimeout,
		interDigitTimeout: DefaultInterDigitTimeout,
		logger:            logger.With("subsystem", "digit-buffer"),
		digits:            make([]byte, 0, 32),
	}
}

// SetFirstDigitTimeout sets the maximum wait time for the first digit.
func (b *DigitBuffer) SetFirstDigitTimeout(d time.Duration) {
	b.firstDigitTimeout = d
}

// SetInterDigitTimeout sets the maximum wait time between consecutive digits.
func (b *DigitBuffer) SetInterDigitTimeout(d time.Duration) {
	b.interDigitTimeout = d
}

// Collect blocks until digit collection is complete, returning the
// accumulated digits and whether the operation timed out. Collection
// completes when:
//   - The first-digit timeout expires (TimedOut=true, Digits="")
//   - The inter-digit timeout expires after receiving digits (TimedOut=true, Digits=partial)
//   - The source channel is closed (returns whatever was collected)
//   - The context is cancelled
//
// This method is safe for concurrent use but typically only one goroutine
// calls Collect at a time per buffer instance.
func (b *DigitBuffer) Collect(ctx context.Context) *DigitBufferResult {
	b.mu.Lock()
	b.digits = b.digits[:0]
	b.mu.Unlock()

	timer := time.NewTimer(b.firstDigitTimeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return &DigitBufferResult{
				Digits:   b.collected(),
				TimedOut: true,
			}

		case digit, ok := <-b.source:
			if !ok {
				// Source channel closed (collector stopped).
				return &DigitBufferResult{
					Digits:   b.collected(),
					TimedOut: false,
				}
			}

			b.mu.Lock()
			b.digits = append(b.digits, digit[0])
			b.lastAt = time.Now()
			b.mu.Unlock()

			b.logger.Debug("digit buffered",
				"digit", digit,
				"buffer", b.collected(),
			)

			// Switch to inter-digit timeout after the first digit.
			if !timer.Stop() {
				// Drain the timer channel if it already fired.
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(b.interDigitTimeout)

		case <-timer.C:
			return &DigitBufferResult{
				Digits:   b.collected(),
				TimedOut: true,
			}
		}
	}
}

// collected returns the current buffer contents as a string.
func (b *DigitBuffer) collected() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.digits)
}

// Buffered returns the number of digits currently in the buffer.
func (b *DigitBuffer) Buffered() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.digits)
}

// Peek returns the current buffer contents without consuming them.
func (b *DigitBuffer) Peek() string {
	return b.collected()
}

// Reset clears the digit buffer.
func (b *DigitBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.digits = b.digits[:0]
}
