package notify

import (
	"context"
	"fmt"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

// Notifier interface for notification adapters.
// Accepts a context so notifications can be cancelled during graceful shutdown.
type Notifier interface {
	Notify(ctx context.Context, message string) error
}

// NewNotifier creates a notifier based on config
func NewNotifier(cfg config.NotifyConfig) (Notifier, error) {
	switch cfg.Type {
	case "slack":
		return NewSlackNotifier(cfg.Slack)
	case "webhook":
		return NewWebhookNotifier(cfg.Webhook)
	case "email":
		return nil, fmt.Errorf("email notifier not yet implemented")
	case "discord":
		return nil, fmt.Errorf("discord notifier not yet implemented")
	case "none":
		return NewNoopNotifier(), nil
	default:
		return nil, fmt.Errorf("unknown notification type: %s", cfg.Type)
	}
}

