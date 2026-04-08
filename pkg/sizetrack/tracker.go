// Package sizetrack records backup file sizes and detects anomalies
// by comparing each new size against a rolling average of recent backups.
package sizetrack

import (
	"math"
	"sync"
)

const (
	// DefaultWindow is the number of recent sizes retained per backup job.
	DefaultWindow = 10

	// DefaultThreshold is the default deviation percentage that triggers an anomaly.
	DefaultThreshold = 50.0

	// MinDataPoints is the minimum number of recorded sizes before anomaly detection kicks in.
	MinDataPoints = 3
)

// Anomaly describes a backup size that deviates significantly from the rolling average.
type Anomaly struct {
	BackupName   string
	CurrentSize  int64
	RollingAvg   int64
	DeviationPct float64
}

// Tracker records backup sizes and detects anomalies per backup job.
// It is safe for concurrent use.
type Tracker struct {
	mu         sync.Mutex
	history    map[string][]int64 // backup name → last N sizes
	thresholds map[string]float64 // backup name → custom threshold
	window     int
}

// NewTracker returns a Tracker with the default rolling window size.
func NewTracker() *Tracker {
	return &Tracker{
		history:    make(map[string][]int64),
		thresholds: make(map[string]float64),
		window:     DefaultWindow,
	}
}

// SetThreshold configures a custom anomaly threshold for a specific backup job.
// The threshold is a percentage (e.g. 30.0 means 30%).
func (t *Tracker) SetThreshold(name string, threshold float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.thresholds[name] = threshold
}

// Record appends sizeBytes to the rolling window for the named backup job
// and returns an *Anomaly if the new size deviates from the rolling average
// by more than the configured threshold. Returns nil when there are fewer
// than 3 data points or the deviation is within the threshold.
func (t *Tracker) Record(name string, sizeBytes int64) *Anomaly {
	t.mu.Lock()
	defer t.mu.Unlock()

	prev := t.history[name]

	// Compute rolling average from existing history (before appending).
	var avg float64
	canDetect := len(prev) >= MinDataPoints
	if canDetect {
		var sum int64
		for _, s := range prev {
			sum += s
		}
		avg = float64(sum) / float64(len(prev))
	}

	// Append the new size and trim to window.
	prev = append(prev, sizeBytes)
	if len(prev) > t.window {
		prev = prev[len(prev)-t.window:]
	}
	t.history[name] = prev

	if !canDetect {
		return nil
	}

	// Determine threshold.
	threshold := DefaultThreshold
	if custom, ok := t.thresholds[name]; ok {
		threshold = custom
	}

	// Calculate deviation percentage.
	if avg == 0 {
		// Avoid division by zero; if average is 0 and current is non-zero, that's 100% deviation.
		if sizeBytes != 0 {
			return &Anomaly{
				BackupName:   name,
				CurrentSize:  sizeBytes,
				RollingAvg:   0,
				DeviationPct: 100.0,
			}
		}
		return nil
	}

	deviation := math.Abs(float64(sizeBytes)-avg) / avg * 100.0

	if deviation > threshold {
		return &Anomaly{
			BackupName:   name,
			CurrentSize:  sizeBytes,
			RollingAvg:   int64(math.Round(avg)),
			DeviationPct: math.Round(deviation*100) / 100, // round to 2 decimal places
		}
	}

	return nil
}
