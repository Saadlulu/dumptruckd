package restore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalLister_ListFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a backup directory structure
	backupDir := filepath.Join(tmpDir, "mybackup", "2024", "01", "15")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create some backup files
	files := []string{
		filepath.Join(backupDir, "dump_testdb_20240115_060000.sql.gz"),
		filepath.Join(backupDir, "dump_testdb_20240115_120000.sql.gz"),
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	lister := NewLocalLister(tmpDir)
	got, err := lister.List(context.Background(), "mybackup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(got), got)
	}
}

func TestLocalLister_EmptyDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create an empty backup directory
	backupDir := filepath.Join(tmpDir, "mybackup")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatal(err)
	}

	lister := NewLocalLister(tmpDir)
	got, err := lister.List(context.Background(), "mybackup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("expected 0 files, got %d: %v", len(got), got)
	}
}

func TestLocalLister_NonexistentPrefix(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	lister := NewLocalLister(tmpDir)
	got, err := lister.List(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != nil {
		t.Fatalf("expected nil for nonexistent prefix, got %v", got)
	}
}

func TestLocalLister_IgnoresNonBackupFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	backupDir := filepath.Join(tmpDir, "mybackup")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a backup file and a non-backup file
	if err := os.WriteFile(filepath.Join(backupDir, "dump.sql.gz"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "readme.txt"), []byte("notes"), 0600); err != nil {
		t.Fatal(err)
	}

	lister := NewLocalLister(tmpDir)
	got, err := lister.List(context.Background(), "mybackup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 backup file, got %d: %v", len(got), got)
	}
}

func TestLocalLister_MultipleExtensions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	backupDir := filepath.Join(tmpDir, "mybackup")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create files with various backup extensions
	testFiles := []string{
		"dump.sql",
		"dump.sql.gz",
		"dump.sql.gz.age",
		"dump.sql.gz.gpg",
	}
	for _, f := range testFiles {
		if err := os.WriteFile(filepath.Join(backupDir, f), []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	lister := NewLocalLister(tmpDir)
	got, err := lister.List(context.Background(), "mybackup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != len(testFiles) {
		t.Fatalf("expected %d files, got %d: %v", len(testFiles), len(got), got)
	}
}
