package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSanitizePath_RejectsDotDot(t *testing.T) {
	t.Parallel()
	_, err := SanitizePath("../../etc/passwd")
	if err == nil {
		t.Error("SanitizePath() should return error for path with '..'")
	}
}

func TestSanitizePath_RejectsTilde(t *testing.T) {
	t.Parallel()
	_, err := SanitizePath("~/secret/file")
	if err == nil {
		t.Error("SanitizePath() should return error for path with '~'")
	}
}

func TestSanitizePath_AbsolutePathStaysAbsolute(t *testing.T) {
	t.Parallel()
	got, err := SanitizePath("/var/backups/dump.sql")
	if err != nil {
		t.Fatalf("SanitizePath() unexpected error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("SanitizePath() should keep absolute paths absolute, got %q", got)
	}
}

func TestSanitizePath_RelativePathStaysRelative(t *testing.T) {
	t.Parallel()
	got, err := SanitizePath("backups/dump.sql")
	if err != nil {
		t.Fatalf("SanitizePath() unexpected error: %v", err)
	}
	if filepath.IsAbs(got) {
		t.Errorf("SanitizePath() should keep relative paths relative, got %q", got)
	}
}

func TestSanitizePath_CleanPath(t *testing.T) {
	t.Parallel()
	got, err := SanitizePath("backups//dump.sql")
	if err != nil {
		t.Fatalf("SanitizePath() unexpected error: %v", err)
	}
	if got != filepath.Join("backups", "dump.sql") {
		t.Errorf("SanitizePath() should clean double slashes, got %q", got)
	}
}

func TestSanitizePath_RejectsNestedTraversal(t *testing.T) {
	t.Parallel()
	_, err := SanitizePath("backups/../../etc/passwd")
	if err == nil {
		t.Error("SanitizePath() should return error for nested traversal")
	}
}

func TestEnsureDir_CreatesNestedDirectories(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	tmpDir := t.TempDir()

	err := EnsureDir(tmpDir)
	if err != nil {
		t.Fatalf("EnsureDir() on existing dir should not error, got %v", err)
	}
}

func TestBuildBackupPath_WithPrefix(t *testing.T) {
	origNow := Now
	Now = func() time.Time { return time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	got, err := BuildBackupPath("backups", "mydb", "dump.sql.gz")
	if err != nil {
		t.Fatalf("BuildBackupPath() unexpected error: %v", err)
	}
	want := filepath.Join("backups", "mydb", "2024/06/15", "dump.sql.gz")
	if got != want {
		t.Errorf("BuildBackupPath() = %q, want %q", got, want)
	}
}

func TestBuildBackupPath_WithoutPrefix(t *testing.T) {
	origNow := Now
	Now = func() time.Time { return time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC) }
	defer func() { Now = origNow }()

	got, err := BuildBackupPath("", "mydb", "dump.sql.gz")
	if err != nil {
		t.Fatalf("BuildBackupPath() unexpected error: %v", err)
	}
	want := filepath.Join("mydb", "2024/06/15", "dump.sql.gz")
	if got != want {
		t.Errorf("BuildBackupPath() = %q, want %q", got, want)
	}
}

func TestBuildBackupPath_RejectsTraversal(t *testing.T) {
	t.Parallel()
	_, err := BuildBackupPath("", "../evil", "dump.sql")
	if err == nil {
		t.Error("BuildBackupPath() should return error for traversal in backup name")
	}
}
