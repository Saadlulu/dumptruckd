package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizePath_StripsDotDot(t *testing.T) {
	got := SanitizePath("../../etc/passwd")
	if strings.Contains(got, "..") {
		t.Errorf("SanitizePath() should strip '..', got %q", got)
	}
}

func TestSanitizePath_StripsTilde(t *testing.T) {
	got := SanitizePath("~/secret/file")
	if strings.Contains(got, "~") {
		t.Errorf("SanitizePath() should strip '~', got %q", got)
	}
}

func TestSanitizePath_AbsolutePathStaysAbsolute(t *testing.T) {
	got := SanitizePath("/var/backups/dump.sql")
	if !filepath.IsAbs(got) {
		t.Errorf("SanitizePath() should keep absolute paths absolute, got %q", got)
	}
}

func TestSanitizePath_RelativePathStaysRelative(t *testing.T) {
	got := SanitizePath("backups/dump.sql")
	if filepath.IsAbs(got) {
		t.Errorf("SanitizePath() should keep relative paths relative, got %q", got)
	}
}

func TestSanitizePath_CleanPath(t *testing.T) {
	got := SanitizePath("backups//dump.sql")
	if strings.Contains(got, "//") {
		t.Errorf("SanitizePath() should clean double slashes, got %q", got)
	}
}

func TestEnsureDir_CreatesNestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "a", "b", "c")

	err := EnsureDir(nested)
	if err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("Directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("EnsureDir() should create a directory")
	}
}

func TestEnsureDir_ExistingDirIsNoOp(t *testing.T) {
	tmpDir := t.TempDir()

	err := EnsureDir(tmpDir)
	if err != nil {
		t.Fatalf("EnsureDir() on existing dir should not error, got %v", err)
	}
}

func TestBuildBackupPath_WithPrefix(t *testing.T) {
	// Override Now for deterministic test
	origNow := Now
	Now = func() time.Time { return time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	got := BuildBackupPath("backups", "mydb", "dump.sql.gz")
	want := filepath.Join("backups", "mydb", "2024/06/15", "dump.sql.gz")
	if got != want {
		t.Errorf("BuildBackupPath() = %q, want %q", got, want)
	}
}

func TestBuildBackupPath_WithoutPrefix(t *testing.T) {
	origNow := Now
	Now = func() time.Time { return time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	got := BuildBackupPath("", "mydb", "dump.sql.gz")
	want := filepath.Join("mydb", "2024/06/15", "dump.sql.gz")
	if got != want {
		t.Errorf("BuildBackupPath() = %q, want %q", got, want)
	}
}
