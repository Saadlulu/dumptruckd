package scheduler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dumptruckd/dumptruckd/pkg/compress"
	"github.com/dumptruckd/dumptruckd/pkg/config"
	"github.com/dumptruckd/dumptruckd/pkg/dump"
	"github.com/dumptruckd/dumptruckd/pkg/notify"
	"github.com/dumptruckd/dumptruckd/pkg/upload"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- Fakes ---

type fakeDumper struct {
	dumpFile string
	dumpErr  error
}

func (f *fakeDumper) Dump(ctx context.Context) (string, error) {
	if f.dumpErr != nil {
		return "", f.dumpErr
	}
	// Create a real temp file so cleanup works
	tmp, _ := os.CreateTemp("", "fake-dump-*.sql")
	tmp.WriteString("fake dump data")
	tmp.Close()
	f.dumpFile = tmp.Name()
	return tmp.Name(), nil
}

type fakeCompressor struct {
	compressErr error
}

func (f *fakeCompressor) Compress(inputPath string) (string, error) {
	if f.compressErr != nil {
		return "", f.compressErr
	}
	return inputPath, nil // passthrough
}

type fakeUploader struct {
	uploadPath string
	uploadErr  error
}

func (f *fakeUploader) Upload(ctx context.Context, filePath string, backupName string) (string, error) {
	if f.uploadErr != nil {
		return "", f.uploadErr
	}
	f.uploadPath = fmt.Sprintf("fake://%s/%s", backupName, filePath)
	return f.uploadPath, nil
}

type fakeNotifier struct {
	messages []string
	err      error
}

func (f *fakeNotifier) Notify(message string) error {
	f.messages = append(f.messages, message)
	return f.err
}

// --- Helper to build a scheduler with fakes ---

func newTestScheduler(cfg *config.Config, fd *fakeDumper, fc *fakeCompressor, fu *fakeUploader, fn *fakeNotifier) *Scheduler {
	s := New(cfg, discardLogger())
	s.WithDumperFactory(func(c config.DatabaseConfig) (dump.Dumper, error) {
		return fd, nil
	}).WithCompressorFactory(func(c config.CompressConfig) (compress.Compressor, error) {
		return fc, nil
	}).WithUploaderFactory(func(c config.UploadConfig) (upload.Uploader, error) {
		return fu, nil
	}).WithNotifierFactory(func(c config.NotifyConfig) (notify.Notifier, error) {
		return fn, nil
	})
	return s
}

func validBackupConfig() config.BackupConfig {
	return config.BackupConfig{
		Name:     "test-backup",
		Schedule: "0 0 * * * *",
		Database: config.DatabaseConfig{Type: "postgres", Host: "localhost", Database: "db"},
		Upload:   config.UploadConfig{Type: "local", Path: "/tmp"},
		Compress: config.CompressConfig{Type: "gzip"},
		Notify:   config.NotifyConfig{Type: "webhook", Webhook: config.WebhookConfig{URL: "http://example.com"}},
	}
}

// --- Tests ---

func TestNew_ReturnsValidScheduler(t *testing.T) {
	cfg := &config.Config{
		Backups: []config.BackupConfig{
			{Name: "test", Schedule: "0 0 * * * *"},
		},
	}

	s := New(cfg, nil)
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.cfg != cfg {
		t.Error("New() did not store config")
	}
	if s.cron == nil {
		t.Error("New() did not create cron instance")
	}
	if s.log == nil {
		t.Error("New() should set a default logger when nil is passed")
	}
	if s.dumperFactory == nil || s.compressorFactory == nil || s.uploaderFactory == nil || s.notifierFactory == nil {
		t.Error("New() should set default factories")
	}
}

func TestStart_InvalidConfig_ReturnsError(t *testing.T) {
	cfg := &config.Config{}
	s := New(cfg, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Start(ctx)
	if err == nil {
		t.Fatal("Start() should return error for invalid config")
	}
}

func TestRunBackup_HappyPath(t *testing.T) {
	fd := &fakeDumper{}
	fc := &fakeCompressor{}
	fu := &fakeUploader{}
	fn := &fakeNotifier{}

	cfg := &config.Config{Backups: []config.BackupConfig{validBackupConfig()}}
	s := newTestScheduler(cfg, fd, fc, fu, fn)

	err := s.RunBackup(context.Background(), cfg.Backups[0])
	if err != nil {
		t.Fatalf("RunBackup() error = %v", err)
	}

	// Notifier should have received a success message
	if len(fn.messages) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(fn.messages))
	}
	if !strings.Contains(fn.messages[0], "completed successfully") {
		t.Errorf("Notification should contain success message, got %q", fn.messages[0])
	}
}

func TestRunBackup_DumpFailure(t *testing.T) {
	fd := &fakeDumper{dumpErr: fmt.Errorf("connection refused")}
	fc := &fakeCompressor{}
	fu := &fakeUploader{}
	fn := &fakeNotifier{}

	cfg := &config.Config{Backups: []config.BackupConfig{validBackupConfig()}}
	s := newTestScheduler(cfg, fd, fc, fu, fn)

	err := s.RunBackup(context.Background(), cfg.Backups[0])
	if err == nil {
		t.Fatal("RunBackup() should error when dump fails")
	}
	if !strings.Contains(err.Error(), "dump failed") {
		t.Errorf("Error should mention dump failure, got %q", err.Error())
	}
}

func TestRunBackup_CompressFailure(t *testing.T) {
	fd := &fakeDumper{}
	fc := &fakeCompressor{compressErr: fmt.Errorf("disk full")}
	fu := &fakeUploader{}
	fn := &fakeNotifier{}

	cfg := &config.Config{Backups: []config.BackupConfig{validBackupConfig()}}
	s := newTestScheduler(cfg, fd, fc, fu, fn)

	err := s.RunBackup(context.Background(), cfg.Backups[0])
	if err == nil {
		t.Fatal("RunBackup() should error when compress fails")
	}
	if !strings.Contains(err.Error(), "compress failed") {
		t.Errorf("Error should mention compress failure, got %q", err.Error())
	}
}

func TestRunBackup_UploadFailure(t *testing.T) {
	fd := &fakeDumper{}
	fc := &fakeCompressor{}
	fu := &fakeUploader{uploadErr: fmt.Errorf("access denied")}
	fn := &fakeNotifier{}

	cfg := &config.Config{Backups: []config.BackupConfig{validBackupConfig()}}
	s := newTestScheduler(cfg, fd, fc, fu, fn)

	err := s.RunBackup(context.Background(), cfg.Backups[0])
	if err == nil {
		t.Fatal("RunBackup() should error when upload fails")
	}
	if !strings.Contains(err.Error(), "upload failed") {
		t.Errorf("Error should mention upload failure, got %q", err.Error())
	}
}

func TestRunBackup_CleansTempFilesOnSuccess(t *testing.T) {
	fd := &fakeDumper{}
	fc := &fakeCompressor{}
	fu := &fakeUploader{}
	fn := &fakeNotifier{}

	cfg := &config.Config{Backups: []config.BackupConfig{validBackupConfig()}}
	s := newTestScheduler(cfg, fd, fc, fu, fn)

	s.RunBackup(context.Background(), cfg.Backups[0])

	// The dump file should be cleaned up
	if fd.dumpFile != "" {
		if _, err := os.Stat(fd.dumpFile); !os.IsNotExist(err) {
			t.Error("Dump file should be cleaned up after successful backup")
		}
	}
}

func TestNotifySuccess_TypeNone_Skips(t *testing.T) {
	fn := &fakeNotifier{}
	cfg := &config.Config{}
	s := newTestScheduler(cfg, &fakeDumper{}, &fakeCompressor{}, &fakeUploader{}, fn)

	s.notifySuccess(config.BackupConfig{
		Name:   "test",
		Notify: config.NotifyConfig{Type: "none"},
	}, "/some/path", 5*time.Second)

	if len(fn.messages) != 0 {
		t.Error("Should not send notification when type is 'none'")
	}
}

func TestNotifySuccess_EmptyType_Skips(t *testing.T) {
	fn := &fakeNotifier{}
	cfg := &config.Config{}
	s := newTestScheduler(cfg, &fakeDumper{}, &fakeCompressor{}, &fakeUploader{}, fn)

	s.notifySuccess(config.BackupConfig{
		Name:   "test",
		Notify: config.NotifyConfig{Type: ""},
	}, "/some/path", 5*time.Second)

	if len(fn.messages) != 0 {
		t.Error("Should not send notification when type is empty")
	}
}

func TestNotifyFailure_TypeNone_Skips(t *testing.T) {
	fn := &fakeNotifier{}
	cfg := &config.Config{}
	s := newTestScheduler(cfg, &fakeDumper{}, &fakeCompressor{}, &fakeUploader{}, fn)

	s.notifyFailure(config.BackupConfig{
		Name:   "test",
		Notify: config.NotifyConfig{Type: "none"},
	}, fmt.Errorf("some error"))

	if len(fn.messages) != 0 {
		t.Error("Should not send notification when type is 'none'")
	}
}

func TestNotifyFailure_PreservesOriginalError(t *testing.T) {
	fn := &fakeNotifier{}
	cfg := &config.Config{}
	s := newTestScheduler(cfg, &fakeDumper{}, &fakeCompressor{}, &fakeUploader{}, fn)

	backupCfg := validBackupConfig()
	originalErr := fmt.Errorf("dump failed: connection refused")

	s.notifyFailure(backupCfg, originalErr)

	if len(fn.messages) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(fn.messages))
	}
	if !strings.Contains(fn.messages[0], "connection refused") {
		t.Errorf("Notification should contain original error, got %q", fn.messages[0])
	}
}

func TestNotifyFailure_NotifierError_DoesNotPanic(t *testing.T) {
	fn := &fakeNotifier{err: fmt.Errorf("slack is down")}
	cfg := &config.Config{}
	s := newTestScheduler(cfg, &fakeDumper{}, &fakeCompressor{}, &fakeUploader{}, fn)

	// Should not panic even when notifier fails
	s.notifyFailure(validBackupConfig(), fmt.Errorf("backup error"))
}
