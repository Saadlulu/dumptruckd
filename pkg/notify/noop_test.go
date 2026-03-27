package notify

import (
	"context"
	"testing"
)

func TestNoopNotifier_Notify_ReturnsNil(t *testing.T) {
	notifier := NewNoopNotifier()
	err := notifier.Notify(context.Background(), "any message")
	if err != nil {
		t.Errorf("Notify() should return nil, got %v", err)
	}
}
