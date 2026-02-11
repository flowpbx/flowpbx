package sip

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRegistrationNotifier_WaitThenNotify(t *testing.T) {
	n := NewRegistrationNotifier()

	var registered bool
	done := make(chan struct{})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		registered = n.WaitForRegistration(ctx, 100)
		close(done)
	}()

	// Give the goroutine time to subscribe.
	time.Sleep(10 * time.Millisecond)

	// Simulate the app registering after receiving the push.
	n.Notify(100)

	<-done
	if !registered {
		t.Error("expected WaitForRegistration to return true after Notify")
	}
}

func TestRegistrationNotifier_Timeout(t *testing.T) {
	n := NewRegistrationNotifier()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	registered := n.WaitForRegistration(ctx, 200)
	if registered {
		t.Error("expected WaitForRegistration to return false on timeout")
	}
}

func TestRegistrationNotifier_NotifyBeforeWait(t *testing.T) {
	n := NewRegistrationNotifier()

	// Notify with no subscribers — should not panic.
	n.Notify(300)

	// Subsequent wait should timeout since the notification was already consumed.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	registered := n.WaitForRegistration(ctx, 300)
	if registered {
		t.Error("expected WaitForRegistration to return false when Notify happened before subscribe")
	}
}

func TestRegistrationNotifier_MultipleWaiters(t *testing.T) {
	n := NewRegistrationNotifier()

	const numWaiters = 5
	results := make([]bool, numWaiters)
	var wg sync.WaitGroup

	for i := 0; i < numWaiters; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			results[idx] = n.WaitForRegistration(ctx, 400)
		}(i)
	}

	// Give goroutines time to subscribe.
	time.Sleep(20 * time.Millisecond)

	// Single Notify should wake all waiters.
	n.Notify(400)

	wg.Wait()

	for i, r := range results {
		if !r {
			t.Errorf("waiter %d: expected true, got false", i)
		}
	}
}

func TestRegistrationNotifier_DifferentExtensions(t *testing.T) {
	n := NewRegistrationNotifier()

	var ext100Registered, ext101Registered bool
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ext100Registered = n.WaitForRegistration(ctx, 100)
	}()

	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		ext101Registered = n.WaitForRegistration(ctx, 101)
	}()

	time.Sleep(20 * time.Millisecond)

	// Only notify extension 100 — extension 101 should timeout.
	n.Notify(100)

	wg.Wait()

	if !ext100Registered {
		t.Error("extension 100: expected registered=true")
	}
	if ext101Registered {
		t.Error("extension 101: expected registered=false (no notification sent)")
	}
}

func TestRegistrationNotifier_SubscribeCancel(t *testing.T) {
	n := NewRegistrationNotifier()

	ch, cancel := n.Subscribe(500)

	// Cancel the subscription.
	cancel()

	// Notify should not block or panic after cancel.
	n.Notify(500)

	// The channel should not have been closed by the cancelled subscription.
	select {
	case <-ch:
		// Channel was closed — that's unexpected since we cancelled.
		// Actually, Notify closes all channels it finds, but cancel removed ours.
		// So we should not receive here.
		t.Error("expected channel to not be closed after cancel")
	default:
		// Expected — channel still open since we cancelled before Notify.
	}
}

// TestRegistrationNotifier_PushWakeFlow simulates the complete push-wake flow:
//  1. INVITE arrives for extension with no registrations
//  2. PBX subscribes to registration events and sends push
//  3. App receives push, wakes up, sends REGISTER
//  4. Registrar calls Notify() which unblocks the waiter
//  5. PBX retries routing and finds the newly registered contact
func TestRegistrationNotifier_PushWakeFlow(t *testing.T) {
	n := NewRegistrationNotifier()

	extensionID := int64(1001)
	pushWaitTimeout := 5 * time.Second

	// Step 1-2: PBX detects no registration, starts waiting.
	waitResult := make(chan bool, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), pushWaitTimeout)
		defer cancel()
		waitResult <- n.WaitForRegistration(ctx, extensionID)
	}()

	// Give the goroutine time to subscribe.
	time.Sleep(20 * time.Millisecond)

	// Step 3-4: Simulate app waking from push and registering.
	// In real flow, this happens ~1-3 seconds after push delivery.
	time.Sleep(50 * time.Millisecond) // Simulate network + SIP registration delay
	n.Notify(extensionID)

	// Step 5: Verify the push-wait was successful.
	select {
	case registered := <-waitResult:
		if !registered {
			t.Error("push-wake flow: expected registration to be received")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("push-wake flow: timed out waiting for result")
	}
}

// TestRegistrationNotifier_PushWakeTimeout simulates the scenario where the
// push is delivered but the app fails to register within the timeout (e.g.,
// app was killed and OS couldn't revive it, or network issues).
func TestRegistrationNotifier_PushWakeTimeout(t *testing.T) {
	n := NewRegistrationNotifier()

	extensionID := int64(2001)
	pushWaitTimeout := 100 * time.Millisecond // Short timeout for test

	ctx, cancel := context.WithTimeout(context.Background(), pushWaitTimeout)
	defer cancel()

	start := time.Now()
	registered := n.WaitForRegistration(ctx, extensionID)
	elapsed := time.Since(start)

	if registered {
		t.Error("expected timeout (no registration)")
	}

	// Verify timeout was approximately the configured duration.
	if elapsed < 80*time.Millisecond {
		t.Errorf("timeout too fast: %v", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("timeout too slow: %v", elapsed)
	}
}
