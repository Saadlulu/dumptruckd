package retry

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDo_SucceedsFirstAttempt(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{MaxRetries: 3, BaseDelay: time.Millisecond}, discardLogger(), "test", func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if calls != 1 {
		t.Errorf("Expected 1 call, got %d", calls)
	}
}

func TestDo_SucceedsAfterRetries(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{MaxRetries: 3, BaseDelay: time.Millisecond}, discardLogger(), "test", func() error {
		calls++
		if calls < 3 {
			return fmt.Errorf("transient error")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if calls != 3 {
		t.Errorf("Expected 3 calls, got %d", calls)
	}
}

func TestDo_FailsAfterAllRetries(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{MaxRetries: 2, BaseDelay: time.Millisecond}, discardLogger(), "upload", func() error {
		calls++
		return fmt.Errorf("permanent error")
	})
	if err == nil {
		t.Fatal("Do() should error after all retries exhausted")
	}
	if calls != 3 { // initial + 2 retries
		t.Errorf("Expected 3 calls, got %d", calls)
	}
}

func TestDo_ZeroRetries_RunsOnce(t *testing.T) {
	calls := 0
	err := Do(context.Background(), Config{MaxRetries: 0, BaseDelay: time.Millisecond}, discardLogger(), "test", func() error {
		calls++
		return fmt.Errorf("error")
	})
	if err == nil {
		t.Fatal("Do() should return error")
	}
	if calls != 1 {
		t.Errorf("Expected 1 call with MaxRetries=0, got %d", calls)
	}
}

func TestDo_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, Config{MaxRetries: 10, BaseDelay: 50 * time.Millisecond}, discardLogger(), "test", func() error {
		calls++
		return fmt.Errorf("keep failing")
	})

	if err == nil {
		t.Fatal("Do() should error when context is cancelled")
	}
	if calls > 3 {
		t.Errorf("Should have stopped retrying after context cancel, got %d calls", calls)
	}
}

func TestDo_NilLogger_DoesNotPanic(t *testing.T) {
	err := Do(context.Background(), Config{MaxRetries: 1, BaseDelay: time.Millisecond}, nil, "test", func() error {
		return fmt.Errorf("error")
	})
	if err == nil {
		t.Fatal("Do() should return error")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.BaseDelay != 2*time.Second {
		t.Errorf("BaseDelay = %v, want 2s", cfg.BaseDelay)
	}
}
