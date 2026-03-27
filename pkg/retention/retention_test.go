package retention

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanup_ZeroDays_DoesNothing(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	testFile := filepath.Join(tmpDir, "old-backup.sql")
	os.WriteFile(testFile, []byte("data"), 0644)

	r := New(tmpDir, 0)
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// File should still exist
	if _, err := os.Stat(testFile); err != nil {
		t.Error("Cleanup() with days=0 should not remove any files")
	}
}

func TestCleanup_RemovesOldFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an "old" file by setting its mod time to 10 days ago
	oldFile := filepath.Join(tmpDir, "old-backup.sql")
	os.WriteFile(oldFile, []byte("old data"), 0644)
	oldTime := time.Now().AddDate(0, 0, -10)
	os.Chtimes(oldFile, oldTime, oldTime)

	r := New(tmpDir, 7) // Keep 7 days
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// Old file should be removed
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Cleanup() should remove files older than retention period")
	}
}

func TestCleanup_KeepsNewFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a recent file (just created, so mod time is now)
	newFile := filepath.Join(tmpDir, "new-backup.sql")
	os.WriteFile(newFile, []byte("new data"), 0644)

	r := New(tmpDir, 7)
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// New file should still exist
	if _, err := os.Stat(newFile); err != nil {
		t.Error("Cleanup() should keep files newer than retention period")
	}
}

func TestCleanup_MixedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Old file
	oldFile := filepath.Join(tmpDir, "old.sql")
	os.WriteFile(oldFile, []byte("old"), 0644)
	oldTime := time.Now().AddDate(0, 0, -30)
	os.Chtimes(oldFile, oldTime, oldTime)

	// New file
	newFile := filepath.Join(tmpDir, "new.sql")
	os.WriteFile(newFile, []byte("new"), 0644)

	r := New(tmpDir, 7)
	r.Cleanup()

	// Old should be gone
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old file should be removed")
	}
	// New should remain
	if _, err := os.Stat(newFile); err != nil {
		t.Error("New file should be kept")
	}
}

func TestCleanup_NonexistentPath(t *testing.T) {
	r := New("/nonexistent/path/that/does/not/exist", 7)
	err := r.Cleanup()
	if err == nil {
		t.Error("Cleanup() should error for nonexistent path")
	}
}

func TestCleanup_NegativeDays_DoesNothing(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "backup.sql")
	os.WriteFile(testFile, []byte("data"), 0644)

	r := New(tmpDir, -1)
	err := r.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if _, err := os.Stat(testFile); err != nil {
		t.Error("Cleanup() with negative days should not remove any files")
	}
}
