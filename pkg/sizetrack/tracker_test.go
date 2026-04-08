package sizetrack

import (
	"sync"
	"testing"
)

func TestNewTracker(t *testing.T) {
	t.Parallel()
	tr := NewTracker()
	if tr == nil {
		t.Fatal("NewTracker() returned nil")
	}
	if tr.window != DefaultWindow {
		t.Errorf("window = %d, want %d", tr.window, DefaultWindow)
	}
}

func TestRecord_InsufficientData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		sizes []int64
	}{
		{"zero recordings", nil},
		{"one recording", []int64{1000}},
		{"two recordings", []int64{1000, 2000}},
		{"three recordings (first 3 calls)", []int64{1000, 2000, 3000}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tr := NewTracker()
			for i, s := range tt.sizes {
				anomaly := tr.Record("backup1", s)
				// The first 3 calls should always return nil because we need
				// at least 3 existing data points before the 4th call can detect.
				if i < MinDataPoints && anomaly != nil {
					t.Errorf("Record() call %d returned anomaly, want nil (insufficient data)", i+1)
				}
			}
		})
	}
}

func TestRecord_NoAnomaly(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	// Seed with 3 consistent sizes.
	sizes := []int64{1000, 1000, 1000}
	for _, s := range sizes {
		tr.Record("db", s)
	}

	// A 4th value within 50% of the average (1000) should not trigger.
	anomaly := tr.Record("db", 1200) // 20% deviation
	if anomaly != nil {
		t.Errorf("Record(1200) returned anomaly %+v, want nil (within threshold)", anomaly)
	}
}

func TestRecord_AnomalyDetected_LargerSize(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	// Seed with 3 consistent sizes (avg = 1000).
	for i := 0; i < 3; i++ {
		tr.Record("db", 1000)
	}

	// 2000 is 100% above the average — should trigger.
	anomaly := tr.Record("db", 2000)
	if anomaly == nil {
		t.Fatal("Record(2000) returned nil, want anomaly")
	}
	if anomaly.BackupName != "db" {
		t.Errorf("BackupName = %q, want %q", anomaly.BackupName, "db")
	}
	if anomaly.CurrentSize != 2000 {
		t.Errorf("CurrentSize = %d, want 2000", anomaly.CurrentSize)
	}
	if anomaly.RollingAvg != 1000 {
		t.Errorf("RollingAvg = %d, want 1000", anomaly.RollingAvg)
	}
	if anomaly.DeviationPct != 100.0 {
		t.Errorf("DeviationPct = %f, want 100.0", anomaly.DeviationPct)
	}
}

func TestRecord_AnomalyDetected_SmallerSize(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	// Seed with 3 consistent sizes (avg = 1000).
	for i := 0; i < 3; i++ {
		tr.Record("db", 1000)
	}

	// 400 is 60% below the average — should trigger at default 50%.
	anomaly := tr.Record("db", 400)
	if anomaly == nil {
		t.Fatal("Record(400) returned nil, want anomaly")
	}
	if anomaly.CurrentSize != 400 {
		t.Errorf("CurrentSize = %d, want 400", anomaly.CurrentSize)
	}
}

func TestRecord_RollingWindowCap(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	// Record 15 sizes — window should cap at 10.
	for i := 0; i < 15; i++ {
		tr.Record("db", int64(1000+i))
	}

	tr.mu.Lock()
	histLen := len(tr.history["db"])
	tr.mu.Unlock()

	if histLen != DefaultWindow {
		t.Errorf("history length = %d, want %d", histLen, DefaultWindow)
	}
}

func TestRecord_RollingWindowContainsMostRecent(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	// Record 12 sizes.
	for i := 1; i <= 12; i++ {
		tr.Record("db", int64(i*100))
	}

	tr.mu.Lock()
	hist := tr.history["db"]
	tr.mu.Unlock()

	// Should contain the last 10: 300, 400, ..., 1200
	if len(hist) != DefaultWindow {
		t.Fatalf("history length = %d, want %d", len(hist), DefaultWindow)
	}
	if hist[0] != 300 {
		t.Errorf("oldest entry = %d, want 300", hist[0])
	}
	if hist[len(hist)-1] != 1200 {
		t.Errorf("newest entry = %d, want 1200", hist[len(hist)-1])
	}
}

func TestSetThreshold_CustomThreshold(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	// Set a tight 10% threshold.
	tr.SetThreshold("db", 10.0)

	// Seed with 3 consistent sizes (avg = 1000).
	for i := 0; i < 3; i++ {
		tr.Record("db", 1000)
	}

	// 1200 is 20% above average — should trigger at 10% threshold.
	anomaly := tr.Record("db", 1200)
	if anomaly == nil {
		t.Fatal("Record(1200) with 10% threshold returned nil, want anomaly")
	}
}

func TestSetThreshold_DefaultUsedWhenNotSet(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	// No custom threshold set — default 50% applies.
	for i := 0; i < 3; i++ {
		tr.Record("db", 1000)
	}

	// 1200 is 20% above average — should NOT trigger at default 50%.
	anomaly := tr.Record("db", 1200)
	if anomaly != nil {
		t.Errorf("Record(1200) with default threshold returned anomaly, want nil")
	}
}

func TestRecord_IndependentBackupJobs(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	// Seed "db1" with consistent sizes.
	for i := 0; i < 3; i++ {
		tr.Record("db1", 1000)
	}

	// "db2" has only 1 recording — should not trigger.
	anomaly := tr.Record("db2", 5000)
	if anomaly != nil {
		t.Errorf("Record for db2 with 1 data point returned anomaly, want nil")
	}

	// "db1" with anomalous size should trigger.
	anomaly = tr.Record("db1", 5000)
	if anomaly == nil {
		t.Fatal("Record for db1 with anomalous size returned nil, want anomaly")
	}
}

func TestRecord_ExactThresholdBoundary(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	// Seed with avg = 1000.
	for i := 0; i < 3; i++ {
		tr.Record("db", 1000)
	}

	// Exactly 50% deviation (1500) should NOT trigger (> threshold, not >=).
	anomaly := tr.Record("db", 1500)
	if anomaly != nil {
		t.Errorf("Record(1500) at exact 50%% boundary returned anomaly, want nil")
	}
}

func TestRecord_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	tr := NewTracker()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(val int64) {
			defer wg.Done()
			tr.Record("db", val)
		}(int64(i * 100))
	}
	wg.Wait()

	tr.mu.Lock()
	histLen := len(tr.history["db"])
	tr.mu.Unlock()

	if histLen > DefaultWindow {
		t.Errorf("history length = %d, exceeds window %d after concurrent writes", histLen, DefaultWindow)
	}
}
