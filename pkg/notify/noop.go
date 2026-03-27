package notify

// NoopNotifier is a no-op notifier that does nothing
type NoopNotifier struct{}

// NewNoopNotifier creates a notifier that silently discards all messages.
func NewNoopNotifier() *NoopNotifier {
	return &NoopNotifier{}
}

func (n *NoopNotifier) Notify(message string) error {
	// Do nothing
	return nil
}

