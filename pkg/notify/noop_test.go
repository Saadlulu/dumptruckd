package notify

import "testing"

func TestNoopNotifier_Notify_ReturnsNil(t *testing.T) {
	notifier := NewNoopNotifier()
	err := notifier.Notify("any message")
	if err != nil {
		t.Errorf("Notify() should return nil, got %v", err)
	}
}
