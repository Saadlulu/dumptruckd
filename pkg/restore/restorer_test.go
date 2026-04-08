package restore

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

// fakeLister implements Lister for testing.
type fakeLister struct {
	files []string
	err   error
}

func (f *fakeLister) List(ctx context.Context, prefix string) ([]string, error) {
	return f.files, f.err
}

// fakeDownloader implements verify.Downloader for testing.
type fakeDownloader struct {
	content []byte
	err     error
}

func (f *fakeDownloader) Download(ctx context.Context, remotePath string, localPath string) error {
	if f.err != nil {
		return f.err
	}
	return os.WriteFile(localPath, f.content, 0600)
}

func testBackupConfig(dbType string) config.BackupConfig {
	return config.BackupConfig{
		Name: "testbackup",
		Database: config.DatabaseConfig{
			Type:     dbType,
			Host:     "localhost",
			Port:     5432,
			Database: "testdb",
			Username: "testuser",
		},
		Upload: config.UploadConfig{
			Type: "local",
			Path: "/tmp/backups",
		},
	}
}

func TestRestore_MissingBackupName(t *testing.T) {
	t.Parallel()
	r := NewRestorer(testBackupConfig("postgres"), &fakeDownloader{}, &fakeLister{}, nil)

	err := r.Restore(context.Background(), RestoreOpts{})
	if err == nil {
		t.Fatal("expected error for missing backup name")
	}
	if got := err.Error(); got != "backup name is required" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestRestore_MissingModeFlag(t *testing.T) {
	t.Parallel()
	r := NewRestorer(testBackupConfig("postgres"), &fakeDownloader{}, &fakeLister{}, nil)

	err := r.Restore(context.Background(), RestoreOpts{BackupName: "mybackup"})
	if err == nil {
		t.Fatal("expected error when neither --latest nor --timestamp specified")
	}
	if got := err.Error(); got != "either --latest or --timestamp must be specified" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestRestore_NoFilesFound(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{files: nil}
	r := NewRestorer(testBackupConfig("postgres"), &fakeDownloader{}, lister, nil)

	err := r.Restore(context.Background(), RestoreOpts{BackupName: "mybackup", Latest: true})
	if err == nil {
		t.Fatal("expected error when no files found")
	}
	if got := err.Error(); got != "backup 'mybackup' not found: no files at prefix" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestRestore_ListerError(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{err: fmt.Errorf("connection refused")}
	r := NewRestorer(testBackupConfig("postgres"), &fakeDownloader{}, lister, nil)

	err := r.Restore(context.Background(), RestoreOpts{BackupName: "mybackup", Latest: true})
	if err == nil {
		t.Fatal("expected error when lister fails")
	}
}

func TestRestore_TimestampNotFound(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{files: []string{
		"/backups/mybackup/2024/01/15/dump_testdb_20240115_120000.sql.gz",
	}}
	r := NewRestorer(testBackupConfig("postgres"), &fakeDownloader{}, lister, nil)

	err := r.Restore(context.Background(), RestoreOpts{
		BackupName: "mybackup",
		Timestamp:  "20240116_120000",
	})
	if err == nil {
		t.Fatal("expected error when timestamp not found")
	}
	if got := err.Error(); got != "backup 'mybackup' not found at timestamp 20240116_120000" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestSelectBackup_Latest(t *testing.T) {
	t.Parallel()
	r := &Restorer{}

	objects := []string{
		"/backups/mybackup/2024/01/14/dump_testdb_20240114_060000.sql.gz",
		"/backups/mybackup/2024/01/15/dump_testdb_20240115_120000.sql.gz",
		"/backups/mybackup/2024/01/13/dump_testdb_20240113_180000.sql.gz",
	}

	got, err := r.selectBackup(objects, RestoreOpts{Latest: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/backups/mybackup/2024/01/15/dump_testdb_20240115_120000.sql.gz"
	if got != want {
		t.Errorf("selectBackup(latest) = %q, want %q", got, want)
	}
}

func TestSelectBackup_Timestamp(t *testing.T) {
	t.Parallel()
	r := &Restorer{}

	objects := []string{
		"/backups/mybackup/2024/01/14/dump_testdb_20240114_060000.sql.gz",
		"/backups/mybackup/2024/01/15/dump_testdb_20240115_120000.sql.gz",
	}

	got, err := r.selectBackup(objects, RestoreOpts{Timestamp: "20240114_060000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/backups/mybackup/2024/01/14/dump_testdb_20240114_060000.sql.gz"
	if got != want {
		t.Errorf("selectBackup(timestamp) = %q, want %q", got, want)
	}
}

func TestRestore_DownloadError(t *testing.T) {
	t.Parallel()
	lister := &fakeLister{files: []string{"/backups/mybackup/dump.sql"}}
	dl := &fakeDownloader{err: fmt.Errorf("network error")}
	r := NewRestorer(testBackupConfig("postgres"), dl, lister, nil)

	err := r.Restore(context.Background(), RestoreOpts{BackupName: "mybackup", Latest: true})
	if err == nil {
		t.Fatal("expected error when download fails")
	}
}

func TestRestore_MissingDBPassword(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv()
	// Create a real SQL file that the downloader will "download"
	lister := &fakeLister{files: []string{"/backups/mybackup/dump.sql"}}
	dl := &fakeDownloader{content: []byte("SELECT 1;")}
	r := NewRestorer(testBackupConfig("postgres"), dl, lister, nil)

	// Ensure DB_PASSWORD is not set
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_PASSWORD_TESTDB", "")

	err := r.Restore(context.Background(), RestoreOpts{BackupName: "mybackup", Latest: true})
	if err == nil {
		t.Fatal("expected error when DB_PASSWORD is missing")
	}
}

func TestRestore_UnsupportedDBType(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv()
	cfg := testBackupConfig("mongodb")
	lister := &fakeLister{files: []string{"/backups/mybackup/dump.sql"}}
	dl := &fakeDownloader{content: []byte("data")}
	r := NewRestorer(cfg, dl, lister, nil)

	t.Setenv("DB_PASSWORD", "secret")

	err := r.Restore(context.Background(), RestoreOpts{BackupName: "mybackup", Latest: true})
	if err == nil {
		t.Fatal("expected error for unsupported db type")
	}
}
