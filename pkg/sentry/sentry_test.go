package sentry

import (
	"testing"
	"time"
)

func TestNewSentryManager(t *testing.T) {
	sm := NewSentryManager()

	if sm.status != StatusRoaming {
		t.Errorf("Initial status = %v, want %v", sm.status, StatusRoaming)
	}
	if sm.graceCount != 0 {
		t.Errorf("Initial graceCount = %d, want 0", sm.graceCount)
	}
	if sm.phoneEverSeen != false {
		t.Errorf("Initial phoneEverSeen = %v, want false", sm.phoneEverSeen)
	}
	if sm.shutdownPending != false {
		t.Errorf("Initial shutdownPending = %v, want false", sm.shutdownPending)
	}
}

func TestSetStatus(t *testing.T) {
	sm := NewSentryManager()

	var callbackStatus SentryStatus
	sm.SetStatusCallback(func(status SentryStatus) {
		callbackStatus = status
	})

	sm.setStatus(StatusMonitoring)

	if sm.status != StatusMonitoring {
		t.Errorf("Status = %v, want %v", sm.status, StatusMonitoring)
	}
	if callbackStatus != StatusMonitoring {
		t.Errorf("Callback received status = %v, want %v", callbackStatus, StatusMonitoring)
	}
}

func TestCancelShutdown(t *testing.T) {
	sm := NewSentryManager()

	// Test cancel when no shutdown pending
	result := sm.CancelShutdown()
	if result != false {
		t.Error("CancelShutdown() should return false when no shutdown pending")
	}

	// Simulate shutdown pending
	sm.mu.Lock()
	sm.shutdownPending = true
	sm.mu.Unlock()

	// Cancel in a goroutine to avoid blocking
	go func() {
		time.Sleep(100 * time.Millisecond)
		result := sm.CancelShutdown()
		if result != true {
			t.Error("CancelShutdown() should return true when shutdown pending")
		}
	}()

	// Wait for the cancel
	select {
	case <-sm.cancelShutdown:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Error("CancelShutdown() did not close the channel")
	}
}

func TestIsShutdownPending(t *testing.T) {
	sm := NewSentryManager()

	if sm.IsShutdownPending() {
		t.Error("IsShutdownPending() should return false initially")
	}

	sm.mu.Lock()
	sm.shutdownPending = true
	sm.mu.Unlock()

	if !sm.IsShutdownPending() {
		t.Error("IsShutdownPending() should return true after setting")
	}
}

func TestStatusConstants(t *testing.T) {
	// Verify all status constants are unique
	statuses := []SentryStatus{
		StatusRoaming,
		StatusMonitoring,
		StatusGracePeriod,
		StatusShutdownImminent,
		StatusPaused,
		StatusWaitingForPhone,
	}

	seen := make(map[SentryStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("Duplicate status value: %v", s)
		}
		seen[s] = true
	}
}
