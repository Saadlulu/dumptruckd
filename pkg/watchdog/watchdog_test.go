package watchdog

import (
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeAlerter struct {
	mu       sync.Mutex
	messages []string
}

func (f *fakeAlerter) Alert(message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, message)
	return nil
}

func (f *fakeAlerter) getMessages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.messages))
	copy(cp, f.messages)
	return cp
}

func TestWatchdog_NoAlertWhenBackupIsRecent(t *testing.T) {
	alerter := &fakeAlerter{}
	w := New(discardLogger(), alerter)

	w.Register("test-backup", 1*time.Hour)
	w.RecordSuccess("test-backup")

	stale := w.CheckAll()
	if len(stale) != 0 {
		t.Errorf("Expected no stale backups, got %v", stale)
	}
	if len(alerter.getMessages()) != 0 {
		t.Error("Should not alert when backup is recent")
	}
}

func TestWatchdog_AlertWhenBackupIsStale(t *testing.T) {
	alerter := &fakeAlerter{}
	w := New(discardLogger(), alerter)

	w.Register("test-backup", 1*time.Hour)

	// Simulate a success that happened 3 hours ago
	w.mu.Lock()
	old := time.Now().Add(-3 * time.Hour)
	w.jobs["test-backup"].lastSuccess = &old
	w.mu.Unlock()

	stale := w.CheckAll()
	if len(stale) != 1 {
		t.Fatalf("Expected 1 stale backup, got %d", len(stale))
	}
	if stale[0] != "test-backup" {
		t.Errorf("Stale backup = %q, want %q", stale[0], "test-backup")
	}
	msgs := alerter.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(msgs))
	}
}

func TestWatchdog_AlertWhenNeverRun(t *testing.T) {
	alerter := &fakeAlerter{}
	w := New(discardLogger(), alerter)

	w.Register("test-backup", 1*time.Hour)

	// Simulate enough time has passed since registration
	w.mu.Lock()
	old := time.Now().Add(-3 * time.Hour)
	w.jobs["test-backup"].registeredAt = old
	w.mu.Unlock()

	stale := w.CheckAll()
	if len(stale) != 1 {
		t.Fatalf("Expected 1 stale backup (never run), got %d", len(stale))
	}
}

func TestWatchdog_NoAlertBeforeFirstExpectedRun(t *testing.T) {
	alerter := &fakeAlerter{}
	w := New(discardLogger(), alerter)

	// Just registered, interval is 1 hour — shouldn't alert yet
	w.Register("test-backup", 1*time.Hour)

	stale := w.CheckAll()
	if len(stale) != 0 {
		t.Errorf("Should not alert before first expected run window, got %v", stale)
	}
}

func TestWatchdog_MultipleJobs(t *testing.T) {
	alerter := &fakeAlerter{}
	w := New(discardLogger(), alerter)

	w.Register("healthy", 1*time.Hour)
	w.Register("stale", 1*time.Hour)

	w.RecordSuccess("healthy")

	// Make "stale" look old
	w.mu.Lock()
	old := time.Now().Add(-3 * time.Hour)
	w.jobs["stale"].registeredAt = old
	w.mu.Unlock()

	stale := w.CheckAll()
	if len(stale) != 1 {
		t.Fatalf("Expected 1 stale backup, got %d", len(stale))
	}
	if stale[0] != "stale" {
		t.Errorf("Stale backup = %q, want %q", stale[0], "stale")
	}
}

func TestWatchdog_ConsecutiveFailuresAlert(t *testing.T) {
	alerter := &fakeAlerter{}
	w := New(discardLogger(), alerter)

	w.Register("failing", 1*time.Hour)
	w.RecordSuccess("failing") // initial success

	// Record 3 consecutive failures
	w.RecordFailure("failing")
	w.RecordFailure("failing")
	w.RecordFailure("failing")

	w.mu.Lock()
	job := w.jobs["failing"]
	w.mu.Unlock()

	if job.consecutiveFails != 3 {
		t.Errorf("consecutiveFails = %d, want 3", job.consecutiveFails)
	}
}

func TestWatchdog_SuccessResetsFailCount(t *testing.T) {
	alerter := &fakeAlerter{}
	w := New(discardLogger(), alerter)

	w.Register("test", 1*time.Hour)
	w.RecordFailure("test")
	w.RecordFailure("test")
	w.RecordSuccess("test")

	w.mu.Lock()
	job := w.jobs["test"]
	w.mu.Unlock()

	if job.consecutiveFails != 0 {
		t.Errorf("consecutiveFails should reset to 0 after success, got %d", job.consecutiveFails)
	}
}

func TestWatchdog_RecordFailureAlerts(t *testing.T) {
	alerter := &fakeAlerter{}
	w := New(discardLogger(), alerter)

	w.Register("test", 1*time.Hour)
	w.RecordFailure("test")

	msgs := alerter.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 alert on failure, got %d", len(msgs))
	}
}
