package watchdog

import (
	"context"
	"log/slog"

	"github.com/Saadlulu/dumptruckd/pkg/notify"
)

// NotifyAlerter wraps a Notifier to implement the Alerter interface.
type NotifyAlerter struct {
	notifier notify.Notifier
	ctx      context.Context
}

// NewNotifyAlerter creates an alerter that sends through a Notifier.
// Uses context.Background() by default; call WithContext to set a cancellable context.
func NewNotifyAlerter(n notify.Notifier) *NotifyAlerter {
	return &NotifyAlerter{notifier: n, ctx: context.Background()}
}

// WithContext returns a copy of the alerter that uses the given context for notifications.
func (a *NotifyAlerter) WithContext(ctx context.Context) *NotifyAlerter {
	return &NotifyAlerter{notifier: a.notifier, ctx: ctx}
}

// Alert sends the message through the wrapped notifier.
func (a *NotifyAlerter) Alert(message string) error {
	return a.notifier.Notify(a.ctx, message)
}

// MultiAlerter fans out alerts to multiple notifiers.
type MultiAlerter struct {
	notifiers []notify.Notifier
	log       *slog.Logger
	ctx       context.Context
}

// NewMultiAlerter creates an alerter that sends through multiple notifiers.
func NewMultiAlerter(notifiers []notify.Notifier, log *slog.Logger, ctx context.Context) *MultiAlerter {
	return &MultiAlerter{notifiers: notifiers, log: log, ctx: ctx}
}

// Alert sends the message through all wrapped notifiers.
func (a *MultiAlerter) Alert(message string) error {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for _, n := range a.notifiers {
		if err := n.Notify(ctx, message); err != nil {
			a.log.Error("alerter notification failed", "error", err)
			lastErr = err
		}
	}
	return lastErr
}

// LogAlerter is a fallback alerter that only logs warnings.
type LogAlerter struct {
	log *slog.Logger
}

// NewLogAlerter creates an alerter that only logs messages.
func NewLogAlerter(log *slog.Logger) *LogAlerter {
	return &LogAlerter{log: log}
}

// Alert logs the message as a warning.
func (a *LogAlerter) Alert(message string) error {
	a.log.Warn(message)
	return nil
}
