package notify

import "context"

// NoopNotifier is a no-op notifier that does nothing
type NoopNotifier struct{}

// NewNoopNotifier creates a notifier that silently discards all messages.
func NewNoopNotifier() *NoopNotifier {
	return &NoopNotifier{}
}

func (n *NoopNotifier) Notify(ctx context.Context, message string) error {
	return nil
}

