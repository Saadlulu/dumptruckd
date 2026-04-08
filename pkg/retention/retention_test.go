package retention

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Saadlulu/dumptruckd/internal/utils"
)

func TestCleanup_ZeroDays_ZeroKeepLast_DoesNothing(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "old-backup.sql")
	_ = os.WriteFile(testFile, []byte("data"), 0644)

	r := New(tmpDir, 0, 0)
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if _, err := os.Stat(testFile); err != nil {
		t.Error("Cleanup() with days=0 and keepLast=0 should not remove any files")
	}
}

func TestCleanup_RemovesOldFiles(t *testing.T) {
	tmpDir := t.TempDir()

	oldFile := filepath.Join(tmpDir, "old-backup.sql")
	_ = os.WriteFile(oldFile, []byte("old data"), 0644)
	oldTime := time.Now().AddDate(0, 0, -10)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	r := New(tmpDir, 7, 0) // Keep 7 days, no count limit
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Cleanup() should remove files older than retention period")
	}
}

func TestCleanup_KeepsNewFiles(t *testing.T) {
	tmpDir := t.TempDir()

	newFile := filepath.Join(tmpDir, "new-backup.sql")
	_ = os.WriteFile(newFile, []byte("new data"), 0644)

	r := New(tmpDir, 7, 0)
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if _, err := os.Stat(newFile); err != nil {
		t.Error("Cleanup() should keep files newer than retention period")
	}
}

func TestCleanup_MixedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	oldFile := filepath.Join(tmpDir, "old.sql")
	_ = os.WriteFile(oldFile, []byte("old"), 0644)
	oldTime := time.Now().AddDate(0, 0, -30)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	newFile := filepath.Join(tmpDir, "new.sql")
	_ = os.WriteFile(newFile, []byte("new"), 0644)

	r := New(tmpDir, 7, 0)
	_ = r.Cleanup()

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old file should be removed")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Error("New file should be kept")
	}
}

func TestCleanup_NonexistentPath(t *testing.T) {
	r := New("/nonexistent/path/that/does/not/exist", 7, 0)
	err := r.Cleanup()
	if err == nil {
		t.Error("Cleanup() should error for nonexistent path")
	}
}

func TestCleanup_NegativeDays_DoesNothing(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "backup.sql")
	_ = os.WriteFile(testFile, []byte("data"), 0644)

	r := New(tmpDir, -1, 0)
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if _, err := os.Stat(testFile); err != nil {
		t.Error("Cleanup() with negative days should not remove any files")
	}
}

// --- KeepLast tests ---

func TestCleanup_KeepLast_OnlyCountBased(t *testing.T) {
	tmpDir := t.TempDir()
	origNow := utils.Now
	defer func() { utils.Now = origNow }()
	utils.Now = func() time.Time { return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC) }

	// Create 5 files with different mod times
	files := []struct {
		name    string
		daysOld int
	}{
		{"backup-1.sql", 5},
		{"backup-2.sql", 4},
		{"backup-3.sql", 3},
		{"backup-4.sql", 2},
		{"backup-5.sql", 1},
	}
	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)
		_ = os.WriteFile(path, []byte("data"), 0644)
		modTime := utils.Now().AddDate(0, 0, -f.daysOld)
		_ = os.Chtimes(path, modTime, modTime)
	}

	// days=0, keepLast=3: only count-based retention
	r := New(tmpDir, 0, 3)
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// The 3 newest files should remain (backup-5, backup-4, backup-3)
	for _, name := range []string{"backup-5.sql", "backup-4.sql", "backup-3.sql"} {
		if _, err := os.Stat(filepath.Join(tmpDir, name)); err != nil {
			t.Errorf("File %s should be kept (within keepLast=3)", name)
		}
	}
	// The 2 oldest should be removed
	for _, name := range []string{"backup-1.sql", "backup-2.sql"} {
		if _, err := os.Stat(filepath.Join(tmpDir, name)); !os.IsNotExist(err) {
			t.Errorf("File %s should be removed (outside keepLast=3)", name)
		}
	}
}

func TestCleanup_KeepLast_UnionWithDays(t *testing.T) {
	tmpDir := t.TempDir()
	origNow := utils.Now
	defer func() { utils.Now = origNow }()
	utils.Now = func() time.Time { return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC) }

	// Create files:
	// backup-1: 20 days old (outside days=7, outside keepLast=2)
	// backup-2: 10 days old (outside days=7, inside keepLast=2 — kept by count)
	// backup-3: 3 days old  (inside days=7, inside keepLast=2 — kept by both)
	files := []struct {
		name    string
		daysOld int
	}{
		{"backup-1.sql", 20},
		{"backup-2.sql", 10},
		{"backup-3.sql", 3},
	}
	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)
		_ = os.WriteFile(path, []byte("data"), 0644)
		modTime := utils.Now().AddDate(0, 0, -f.daysOld)
		_ = os.Chtimes(path, modTime, modTime)
	}

	r := New(tmpDir, 7, 2)
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// backup-3: kept by days AND count
	if _, err := os.Stat(filepath.Join(tmpDir, "backup-3.sql")); err != nil {
		t.Error("backup-3.sql should be kept (within days=7 and keepLast=2)")
	}
	// backup-2: kept by count (top 2 newest)
	if _, err := os.Stat(filepath.Join(tmpDir, "backup-2.sql")); err != nil {
		t.Error("backup-2.sql should be kept (within keepLast=2)")
	}
	// backup-1: removed (outside both policies)
	if _, err := os.Stat(filepath.Join(tmpDir, "backup-1.sql")); !os.IsNotExist(err) {
		t.Error("backup-1.sql should be removed (outside days=7 and keepLast=2)")
	}
}

func TestCleanup_KeepLast_ZeroIgnoresCount(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an old file
	oldFile := filepath.Join(tmpDir, "old-backup.sql")
	_ = os.WriteFile(oldFile, []byte("data"), 0644)
	oldTime := time.Now().AddDate(0, 0, -30)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	// days=7, keepLast=0: only days-based retention
	r := New(tmpDir, 7, 0)
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old file should be removed when keepLast=0 (days-only policy)")
	}
}

func TestCleanup_KeepLast_MoreThanFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 2 files, keepLast=5 — all should be kept
	for _, name := range []string{"a.sql", "b.sql"} {
		_ = os.WriteFile(filepath.Join(tmpDir, name), []byte("data"), 0644)
	}

	r := New(tmpDir, 0, 5)
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	for _, name := range []string{"a.sql", "b.sql"} {
		if _, err := os.Stat(filepath.Join(tmpDir, name)); err != nil {
			t.Errorf("File %s should be kept (keepLast=5 > file count)", name)
		}
	}
}

func TestCleanup_DaysOnly_OldFileKeptByCount(t *testing.T) {
	tmpDir := t.TempDir()
	origNow := utils.Now
	defer func() { utils.Now = origNow }()
	utils.Now = func() time.Time { return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC) }

	// Single old file, days=1, keepLast=1 — kept by count even though old
	oldFile := filepath.Join(tmpDir, "only-backup.sql")
	_ = os.WriteFile(oldFile, []byte("data"), 0644)
	oldTime := utils.Now().AddDate(0, 0, -30)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	r := New(tmpDir, 1, 1)
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if _, err := os.Stat(oldFile); err != nil {
		t.Error("Old file should be kept because it's within keepLast=1")
	}
}
