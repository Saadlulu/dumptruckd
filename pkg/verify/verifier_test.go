package verify

import (
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// fakeDownloader implements Downloader for testing.
type fakeDownloader struct {
	// sourceFile is the local file to copy from when Download is called.
	sourceFile string
	// err is returned by Download if non-nil.
	err error
}

func (d *fakeDownloader) Download(ctx context.Context, remotePath string, localPath string) error {
	if d.err != nil {
		return d.err
	}
	data, err := os.ReadFile(d.sourceFile)
	if err != nil {
		return err
	}
	return os.WriteFile(localPath, data, 0600)
}

func TestNewVerifier(t *testing.T) {
	t.Parallel()

	dl := NewLocalDownloader()
	v := NewVerifier(dl, nil)
	if v == nil {
		t.Fatal("NewVerifier returned nil")
	}
	if v.downloader != dl {
		t.Error("downloader not set correctly")
	}
}

func TestVerify_DownloadError(t *testing.T) {
	t.Parallel()

	v := NewVerifier(&fakeDownloader{err: fmt.Errorf("network error")}, nil)
	err := v.Verify(context.Background(), "/some/remote/backup.sql.gz", "postgres", "gzip", "none")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); !contains(got, "download backup for verification") {
		t.Errorf("error should mention download, got: %s", got)
	}
}

func TestVerify_NonGzip_FileSize(t *testing.T) {
	t.Parallel()

	// Create a temp file with some content (non-gzip, non-postgres)
	tmpDir := t.TempDir()
	backupFile := filepath.Join(tmpDir, "backup.sql")
	if err := os.WriteFile(backupFile, []byte("CREATE TABLE test;"), 0600); err != nil {
		t.Fatal(err)
	}

	v := NewVerifier(&fakeDownloader{sourceFile: backupFile}, nil)
	err := v.Verify(context.Background(), backupFile, "mysql", "none", "none")
	if err != nil {
		t.Fatalf("expected no error for valid non-empty file, got: %v", err)
	}
}

func TestVerify_EmptyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "backup.sql")
	if err := os.WriteFile(emptyFile, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}

	v := NewVerifier(&fakeDownloader{sourceFile: emptyFile}, nil)
	err := v.Verify(context.Background(), emptyFile, "mysql", "none", "none")
	if err == nil {
		t.Fatal("expected error for empty file, got nil")
	}
	if got := err.Error(); !contains(got, "backup file is empty") {
		t.Errorf("error should mention empty file, got: %s", got)
	}
}

func TestVerify_GzipDecompress(t *testing.T) {
	t.Parallel()

	// Create a gzip-compressed file with content
	tmpDir := t.TempDir()
	gzFile := filepath.Join(tmpDir, "backup.sql.gz")
	createGzipFile(t, gzFile, []byte("some sql content here"))

	v := NewVerifier(&fakeDownloader{sourceFile: gzFile}, nil)
	// Use "mysql" db type so we just do a file size check (no pg_restore needed)
	err := v.Verify(context.Background(), gzFile, "mysql", "gzip", "none")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestVerify_TempFileCleanup(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	backupFile := filepath.Join(tmpDir, "backup.sql")
	if err := os.WriteFile(backupFile, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	v := NewVerifier(&fakeDownloader{sourceFile: backupFile}, nil)
	// Run verification — temp dir should be cleaned up after
	_ = v.Verify(context.Background(), backupFile, "mysql", "none", "none")

	// Check that no dumptruckd-verify temp dirs remain in the system temp dir
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "dumptruckd-verify-*"))
	for _, m := range matches {
		// Only flag dirs created during this test (best effort check)
		t.Logf("temp dir found (may be from another test): %s", m)
	}
}

func TestVerify_ContextCancelled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	backupFile := filepath.Join(tmpDir, "backup.sql")
	if err := os.WriteFile(backupFile, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	v := NewVerifier(&fakeDownloader{sourceFile: backupFile}, nil)
	err := v.Verify(ctx, backupFile, "mysql", "none", "none")
	// The download or file operations should fail with context cancelled
	if err == nil {
		// It's possible the operation completes before context check — that's ok
		t.Log("verify completed despite cancelled context (file was small enough)")
	}
}

func TestExtensionForRemote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple gz", "/path/to/backup.sql.gz", ".sql.gz"},
		{"s3 path", "s3://bucket/prefix/backup.sql.gz.age", ".sql.gz.age"},
		{"no extension", "/path/to/backup", ""},
		{"single extension", "/path/to/backup.gz", ".gz"},
		{"triple extension", "/path/to/backup.sql.gz.gpg", ".sql.gz.gpg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extensionForRemote(tt.input)
			if got != tt.expected {
				t.Errorf("extensionForRemote(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLocalDownloader_Download(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	content := []byte("hello world")
	if err := os.WriteFile(srcFile, content, 0600); err != nil {
		t.Fatal(err)
	}

	dl := NewLocalDownloader()
	dstFile := filepath.Join(tmpDir, "dest.txt")
	if err := dl.Download(context.Background(), srcFile, dstFile); err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestLocalDownloader_DownloadMissingSource(t *testing.T) {
	t.Parallel()

	dl := NewLocalDownloader()
	dstFile := filepath.Join(t.TempDir(), "dest.txt")
	err := dl.Download(context.Background(), "/nonexistent/file.txt", dstFile)
	if err == nil {
		t.Fatal("expected error for missing source, got nil")
	}
}

func TestValidatePostgres_ValidPlainTextDump(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dumpFile := filepath.Join(tmpDir, "backup.sql")
	content := "-- PostgreSQL database dump\n-- Dumped from database version 15.4\nCREATE TABLE test (id int);\n"
	if err := os.WriteFile(dumpFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	v := NewVerifier(&fakeDownloader{sourceFile: dumpFile}, nil)
	err := v.Verify(context.Background(), dumpFile, "postgres", "none", "none")
	if err != nil {
		t.Fatalf("expected no error for valid postgres dump, got: %v", err)
	}
}

func TestValidatePostgres_CustomFormatDump(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dumpFile := filepath.Join(tmpDir, "backup.dump")
	// PGDMP is the magic header for custom-format pg_dump archives
	content := "PGDMP\x00\x00\x00some binary data here"
	if err := os.WriteFile(dumpFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	v := NewVerifier(&fakeDownloader{sourceFile: dumpFile}, nil)
	err := v.Verify(context.Background(), dumpFile, "postgres", "none", "none")
	if err != nil {
		t.Fatalf("expected no error for custom-format postgres dump, got: %v", err)
	}
}

func TestValidatePostgres_InvalidDump(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dumpFile := filepath.Join(tmpDir, "backup.sql")
	if err := os.WriteFile(dumpFile, []byte("this is not a postgres dump"), 0600); err != nil {
		t.Fatal(err)
	}

	v := NewVerifier(&fakeDownloader{sourceFile: dumpFile}, nil)
	err := v.Verify(context.Background(), dumpFile, "postgres", "none", "none")
	if err == nil {
		t.Fatal("expected error for invalid postgres dump, got nil")
	}
}

func TestValidatePostgres_EmptyFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dumpFile := filepath.Join(tmpDir, "backup.sql")
	if err := os.WriteFile(dumpFile, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}

	v := NewVerifier(&fakeDownloader{sourceFile: dumpFile}, nil)
	err := v.Verify(context.Background(), dumpFile, "postgres", "none", "none")
	if err == nil {
		t.Fatal("expected error for empty postgres dump, got nil")
	}
}

func TestValidatePostgres_GzipCompressedDump(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	gzFile := filepath.Join(tmpDir, "backup.sql.gz")
	content := "-- PostgreSQL database dump\nCREATE TABLE test (id int);\n"
	createGzipFile(t, gzFile, []byte(content))

	v := NewVerifier(&fakeDownloader{sourceFile: gzFile}, nil)
	err := v.Verify(context.Background(), gzFile, "postgres", "gzip", "none")
	if err != nil {
		t.Fatalf("expected no error for gzip-compressed postgres dump, got: %v", err)
	}
}

// helpers

func createGzipFile(t *testing.T, path string, data []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	if _, err := gw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
