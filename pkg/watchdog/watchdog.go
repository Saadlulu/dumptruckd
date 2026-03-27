package watchdog

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Saadlulu/dumptruckd/internal/utils"
)

// Alerter sends alert messages when backups are stale or failing.
type Alerter interface {
	Alert(message string) error
}

type jobState struct {
	name             string
	interval         time.Duration
	lastSuccess      *time.Time
	lastFailure      *time.Time
	lastError        string
	registeredAt     time.Time
	consecutiveFails int
}

// Watchdog monitors backup jobs and alerts when they go stale or fail repeatedly.
type Watchdog struct {
	log      *slog.Logger
	alerter  Alerter
	mu       sync.Mutex
	jobs     map[string]*jobState
	stopCh   chan struct{}
	stopOnce sync.Once
}

// New creates a new Watchdog with the given logger and alerter.
func New(log *slog.Logger, alerter Alerter) *Watchdog {
	if log == nil {
		log = slog.Default()
	}
	return &Watchdog{
		log:     log,
		alerter: alerter,
		jobs:    make(map[string]*jobState),
		stopCh:  make(chan struct{}),
	}
}

// Register adds a backup job to be monitored with its expected run interval.
func (w *Watchdog) Register(name string, interval time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.jobs[name] = &jobState{
		name:         name,
		interval:     interval,
		registeredAt: utils.Now(),
	}
	w.log.Debug("watchdog registered job", "backup", name, "interval", interval)
}

// RecordSuccess records a successful backup and resets the failure counter.
func (w *Watchdog) RecordSuccess(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	job, ok := w.jobs[name]
	if !ok {
		return
	}
	now := utils.Now()
	job.lastSuccess = &now
	job.consecutiveFails = 0
	job.lastError = ""
}

// RecordFailure records a failed backup and sends an immediate alert.
func (w *Watchdog) RecordFailure(name string) {
	w.mu.Lock()
	job, ok := w.jobs[name]
	if !ok {
		w.mu.Unlock()
		return
	}
	now := utils.Now()
	job.lastFailure = &now
	job.consecutiveFails++
	fails := job.consecutiveFails
	w.mu.Unlock()

	msg := fmt.Sprintf("🚨 Backup '%s' failed (attempt #%d)", name, fails)
	w.sendAlert(msg)
}

// CheckAll checks all registered jobs for staleness.
// Returns the names of stale backups.
func (w *Watchdog) CheckAll() []string {
	w.mu.Lock()

	var stale []string
	var alerts []string
	now := utils.Now()

	for name, job := range w.jobs {
		deadline := job.interval * 2

		if job.lastSuccess != nil {
			if now.Sub(*job.lastSuccess) > deadline {
				stale = append(stale, name)
				alerts = append(alerts, fmt.Sprintf("🚨 Backup '%s' is stale — last success was %s ago (expected every %s)",
					name, now.Sub(*job.lastSuccess).Round(time.Minute), job.interval))
			}
		} else {
			if now.Sub(job.registeredAt) > deadline {
				stale = append(stale, name)
				alerts = append(alerts, fmt.Sprintf("🚨 Backup '%s' has never completed successfully — registered %s ago",
					name, now.Sub(job.registeredAt).Round(time.Minute)))
			}
		}
	}

	w.mu.Unlock()

	for _, msg := range alerts {
		w.sendAlert(msg)
	}

	return stale
}

// StartPeriodicCheck runs CheckAll on a timer. Non-blocking.
func (w *Watchdog) StartPeriodicCheck(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stale := w.CheckAll()
				if len(stale) > 0 {
					w.log.Warn("stale backups detected", "count", len(stale), "backups", stale)
				}
			case <-w.stopCh:
				return
			}
		}
	}()
}

// Stop stops the periodic check loop. Safe to call multiple times.
func (w *Watchdog) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
}

func (w *Watchdog) sendAlert(msg string) {
	if w.alerter == nil {
		return
	}
	w.log.Warn(msg)
	if err := w.alerter.Alert(msg); err != nil {
		w.log.Error("watchdog alert failed", "error", err)
	}
}
