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
	lastAlertLevel   int // tracks escalation: 0=none, 1=warn, 2=error, 3=critical
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
	job.lastAlertLevel = 0
}

// RecordFailure records a failed backup and sends an immediate alert.
// Alert severity escalates with consecutive failures:
//   - 1-2 failures: WARN
//   - 3-5 failures: ERROR
//   - 6+  failures: CRITICAL (with remediation guidance)
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

	var msg string
	switch {
	case fails >= 6:
		msg = fmt.Sprintf("CRITICAL: Backup '%s' has failed %d consecutive times. "+
			"Manual intervention required. Check upload path permissions and database connectivity.",
			name, fails)
	case fails >= 3:
		msg = fmt.Sprintf("ERROR: Backup '%s' has failed %d consecutive times. "+
			"Investigate before the next scheduled run.",
			name, fails)
	default:
		msg = fmt.Sprintf("ALERT: Backup '%s' failed (attempt #%d)", name, fails)
	}
	w.sendAlert(msg)
}

// CheckAll checks all registered jobs for staleness.
// Returns the names of stale backups.
// Alert severity escalates over time to avoid repeating the same WARN indefinitely:
//   - First detection: WARN
//   - After 2x interval without success: ERROR
//   - After 4x interval without success: CRITICAL (with remediation guidance)
func (w *Watchdog) CheckAll() []string {
	w.mu.Lock()

	var stale []string
	type alertMsg struct {
		msg   string
		level int
	}
	var alerts []alertMsg
	now := utils.Now()

	for name, job := range w.jobs {
		deadline := job.interval * 2

		var staleSince time.Duration
		isMissing := false

		if job.lastSuccess != nil {
			staleSince = now.Sub(*job.lastSuccess)
			if staleSince > deadline {
				isMissing = true
			}
		} else {
			staleSince = now.Sub(job.registeredAt)
			if staleSince > deadline {
				isMissing = true
			}
		}

		if !isMissing {
			continue
		}

		stale = append(stale, name)

		// Determine escalation level based on how long the backup has been stale
		var level int
		switch {
		case staleSince > job.interval*4:
			level = 3 // CRITICAL
		case staleSince > job.interval*2:
			level = 2 // ERROR
		default:
			level = 1 // WARN
		}

		// Only alert if the level has escalated since the last alert
		if level <= job.lastAlertLevel {
			continue
		}
		job.lastAlertLevel = level

		var msg string
		switch {
		case job.lastSuccess != nil && level >= 3:
			msg = fmt.Sprintf("CRITICAL: Backup '%s' is stale — last success was %s ago (expected every %s). "+
				"Manual intervention required. Check upload path permissions and database connectivity.",
				name, staleSince.Round(time.Minute), job.interval)
		case job.lastSuccess != nil:
			msg = fmt.Sprintf("ALERT: Backup '%s' is stale — last success was %s ago (expected every %s)",
				name, staleSince.Round(time.Minute), job.interval)
		case level >= 3:
			msg = fmt.Sprintf("CRITICAL: Backup '%s' has never completed successfully — registered %s ago. "+
				"Manual intervention required. Check upload path permissions and database connectivity.",
				name, staleSince.Round(time.Minute))
		default:
			msg = fmt.Sprintf("ALERT: Backup '%s' has never completed successfully — registered %s ago",
				name, staleSince.Round(time.Minute))
		}
		alerts = append(alerts, alertMsg{msg: msg, level: level})
	}

	w.mu.Unlock()

	for _, a := range alerts {
		w.sendAlert(a.msg)
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
