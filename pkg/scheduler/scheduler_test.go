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

	"github.com/Saadlulu/dumptruckd/internal/retry"
	"github.com/Saadlulu/dumptruckd/pkg/compress"
	"github.com/Saadlulu/dumptruckd/pkg/config"
	"github.com/Saadlulu/dumptruckd/pkg/dump"
	"github.com/Saadlulu/dumptruckd/pkg/notify"
	"github.com/Saadlulu/dumptruckd/pkg/upload"
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
	_, _ = tmp.WriteString("fake dump data")
	tmp.Close()
	f.dumpFile = tmp.Name()
	return tmp.Name(), nil
}

type fakeCompressor struct {
	compressErr error
}

func (f *fakeCompressor) Compress(ctx context.Context, inputPath string) (string, error) {
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

func (f *fakeNotifier) Notify(ctx context.Context, message string) error {
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

	_ = s.RunBackup(context.Background(), cfg.Backups[0])

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

	s.notifySuccess(context.Background(), config.BackupConfig{
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

	s.notifySuccess(context.Background(), config.BackupConfig{
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

	s.notifyFailure(context.Background(), config.BackupConfig{
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

	s.notifyFailure(context.Background(), backupCfg, originalErr)

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
	s.notifyFailure(context.Background(), validBackupConfig(), fmt.Errorf("backup error"))
}

// --- Additional fakes for new tests ---

type ctxAwareDumper struct{}

func (d *ctxAwareDumper) Dump(ctx context.Context) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return "", fmt.Errorf("should not reach here")
}

type countingUploaderFactory struct {
	callCount int
	uploader  *fakeUploader
}

func (f *countingUploaderFactory) create(cfg config.UploadConfig) (upload.Uploader, error) {
	f.callCount++
	return f.uploader, nil
}

// --- New Tests ---

func TestRunBackup_CleansTempFilesOnDumpFailure(t *testing.T) {
	t.Parallel()

	// Create a real temp file that the dumper will "produce" before failing
	tmp, err := os.CreateTemp("", "dump-fail-cleanup-*.sql")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	_, _ = tmp.WriteString("partial dump data")
	tmp.Close()
	tmpPath := tmp.Name()

	fc := &fakeCompressor{}
	fu := &fakeUploader{}
	fn := &fakeNotifier{}

	cfg := &config.Config{Backups: []config.BackupConfig{validBackupConfig()}}
	s := New(cfg, discardLogger())
	s.WithDumperFactory(func(c config.DatabaseConfig) (dump.Dumper, error) {
		// Return a dumper whose Dump() returns (filePath, error) so the
		// retry cleanup branch (dumpFile != "" && dumpErr != nil) fires.
		return &fileThenErrorDumper{path: tmpPath}, nil
	}).WithCompressorFactory(func(c config.CompressConfig) (compress.Compressor, error) {
		return fc, nil
	}).WithUploaderFactory(func(c config.UploadConfig) (upload.Uploader, error) {
		return fu, nil
	}).WithNotifierFactory(func(c config.NotifyConfig) (notify.Notifier, error) {
		return fn, nil
	})
	// Use zero retries so the test completes quickly
	s.WithRetryConfig(retry.Config{MaxRetries: 0, BaseDelay: 0})

	_ = s.RunBackup(context.Background(), cfg.Backups[0])

	// The temp file should have been cleaned up by the retry func's error branch
	if _, statErr := os.Stat(tmpPath); !os.IsNotExist(statErr) {
		os.Remove(tmpPath) // cleanup in case assertion fails
		t.Error("temp file should be cleaned up after dump failure")
	}
}

// fileThenErrorDumper returns a real file path AND an error so the cleanup branch fires.
type fileThenErrorDumper struct {
	path string
}

func (d *fileThenErrorDumper) Dump(ctx context.Context) (string, error) {
	return d.path, fmt.Errorf("connection reset")
}

func TestRunBackup_ContextCancellation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Backups: []config.BackupConfig{validBackupConfig()}}
	s := New(cfg, discardLogger())
	s.WithDumperFactory(func(c config.DatabaseConfig) (dump.Dumper, error) {
		return &ctxAwareDumper{}, nil
	}).WithCompressorFactory(func(c config.CompressConfig) (compress.Compressor, error) {
		return &fakeCompressor{}, nil
	}).WithUploaderFactory(func(c config.UploadConfig) (upload.Uploader, error) {
		return &fakeUploader{}, nil
	}).WithNotifierFactory(func(c config.NotifyConfig) (notify.Notifier, error) {
		return &fakeNotifier{}, nil
	})
	s.WithRetryConfig(retry.Config{MaxRetries: 0, BaseDelay: 0})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling RunBackup

	err := s.RunBackup(ctx, cfg.Backups[0])
	if err == nil {
		t.Fatal("RunBackup() should error when context is cancelled")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("error should mention context cancellation, got %q", err.Error())
	}
}

func TestValidateAdapters_HappyPath(t *testing.T) {
	t.Parallel()

	fd := &fakeDumper{}
	fc := &fakeCompressor{}
	fu := &fakeUploader{}
	fn := &fakeNotifier{}

	cfg := &config.Config{}
	s := newTestScheduler(cfg, fd, fc, fu, fn)

	err := s.ValidateAdapters(validBackupConfig())
	if err != nil {
		t.Fatalf("ValidateAdapters() unexpected error: %v", err)
	}
}

func TestValidateAdapters_DumperFailure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	s := New(cfg, discardLogger())
	s.WithDumperFactory(func(c config.DatabaseConfig) (dump.Dumper, error) {
		return nil, fmt.Errorf("unsupported database type")
	}).WithCompressorFactory(func(c config.CompressConfig) (compress.Compressor, error) {
		return &fakeCompressor{}, nil
	}).WithUploaderFactory(func(c config.UploadConfig) (upload.Uploader, error) {
		return &fakeUploader{}, nil
	}).WithNotifierFactory(func(c config.NotifyConfig) (notify.Notifier, error) {
		return &fakeNotifier{}, nil
	})

	err := s.ValidateAdapters(validBackupConfig())
	if err == nil {
		t.Fatal("ValidateAdapters() should error when dumper factory fails")
	}
	if !strings.Contains(err.Error(), "dumper") {
		t.Errorf("error should mention 'dumper', got %q", err.Error())
	}
}

func TestValidateAdapters_UploaderFailure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	s := New(cfg, discardLogger())
	s.WithDumperFactory(func(c config.DatabaseConfig) (dump.Dumper, error) {
		return &fakeDumper{}, nil
	}).WithCompressorFactory(func(c config.CompressConfig) (compress.Compressor, error) {
		return &fakeCompressor{}, nil
	}).WithUploaderFactory(func(c config.UploadConfig) (upload.Uploader, error) {
		return nil, fmt.Errorf("invalid S3 credentials")
	}).WithNotifierFactory(func(c config.NotifyConfig) (notify.Notifier, error) {
		return &fakeNotifier{}, nil
	})

	err := s.ValidateAdapters(validBackupConfig())
	if err == nil {
		t.Fatal("ValidateAdapters() should error when uploader factory fails")
	}
	if !strings.Contains(err.Error(), "uploader") {
		t.Errorf("error should mention 'uploader', got %q", err.Error())
	}
}

func TestGetOrCreateUploader_CachesUploader(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	fu := &fakeUploader{}
	factory := &countingUploaderFactory{uploader: fu}

	s := New(cfg, discardLogger())
	s.WithUploaderFactory(factory.create)

	uploadCfg := config.UploadConfig{Type: "local", Path: "/tmp/backups"}

	u1, err := s.getOrCreateUploader(uploadCfg)
	if err != nil {
		t.Fatalf("first getOrCreateUploader() error: %v", err)
	}

	u2, err := s.getOrCreateUploader(uploadCfg)
	if err != nil {
		t.Fatalf("second getOrCreateUploader() error: %v", err)
	}

	if u1 != u2 {
		t.Error("getOrCreateUploader() should return the same instance for the same config")
	}
	if factory.callCount != 1 {
		t.Errorf("factory should be called once, got %d", factory.callCount)
	}
}

func TestGetOrCreateUploader_DifferentConfigsGetDifferentUploaders(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	s := New(cfg, discardLogger())
	s.WithUploaderFactory(func(c config.UploadConfig) (upload.Uploader, error) {
		return &fakeUploader{}, nil
	})

	cfg1 := config.UploadConfig{Type: "local", Path: "/tmp/backups-a"}
	cfg2 := config.UploadConfig{Type: "local", Path: "/tmp/backups-b"}

	u1, err := s.getOrCreateUploader(cfg1)
	if err != nil {
		t.Fatalf("getOrCreateUploader(cfg1) error: %v", err)
	}

	u2, err := s.getOrCreateUploader(cfg2)
	if err != nil {
		t.Fatalf("getOrCreateUploader(cfg2) error: %v", err)
	}

	if u1 == u2 {
		t.Error("getOrCreateUploader() should return different instances for different configs")
	}
}

func TestRunRetention_SkipsNonLocal(t *testing.T) {
	t.Parallel()

	fd := &fakeDumper{}
	fc := &fakeCompressor{}
	fu := &fakeUploader{}
	fn := &fakeNotifier{}

	cfg := &config.Config{}
	s := newTestScheduler(cfg, fd, fc, fu, fn)

	backupCfg := validBackupConfig()
	backupCfg.Upload.Type = "s3"
	backupCfg.Retention = config.RetentionConfig{Days: 7}

	// Should not panic or do anything for non-local upload types
	s.runRetention(backupCfg)
}

func TestRunRetention_SkipsZeroDays(t *testing.T) {
	t.Parallel()

	fd := &fakeDumper{}
	fc := &fakeCompressor{}
	fu := &fakeUploader{}
	fn := &fakeNotifier{}

	cfg := &config.Config{}
	s := newTestScheduler(cfg, fd, fc, fu, fn)

	backupCfg := validBackupConfig()
	backupCfg.Upload.Type = "local"
	backupCfg.Retention = config.RetentionConfig{Days: 0}

	// Should not panic or do anything when retention days is 0
	s.runRetention(backupCfg)
}

func TestScheduleBackup_JobLockPreventsDoubleRun(t *testing.T) {
	t.Parallel()

	fd := &fakeDumper{}
	fc := &fakeCompressor{}
	fu := &fakeUploader{}
	fn := &fakeNotifier{}

	cfg := &config.Config{}
	s := newTestScheduler(cfg, fd, fc, fu, fn)

	// getJobLock returns the same mutex for the same name
	mu1 := s.getJobLock("backup-a")
	mu2 := s.getJobLock("backup-a")
	if mu1 != mu2 {
		t.Error("getJobLock() should return the same mutex for the same job name")
	}

	// getJobLock returns different mutexes for different names
	mu3 := s.getJobLock("backup-b")
	if mu1 == mu3 {
		t.Error("getJobLock() should return different mutexes for different job names")
	}

	// Verify TryLock behavior: if locked, TryLock returns false
	mu1.Lock()
	if mu1.TryLock() {
		mu1.Unlock()
		t.Error("TryLock() should return false when mutex is already locked")
	}
	mu1.Unlock()
}
