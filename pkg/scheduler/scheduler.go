package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/dumptruckd/dumptruckd/pkg/compress"
	"github.com/dumptruckd/dumptruckd/pkg/config"
	"github.com/dumptruckd/dumptruckd/pkg/dump"
	"github.com/dumptruckd/dumptruckd/pkg/notify"
	"github.com/dumptruckd/dumptruckd/pkg/upload"
	"github.com/robfig/cron/v3"
)

// DumperFactory creates a Dumper from config.
type DumperFactory func(config.DatabaseConfig) (dump.Dumper, error)

// CompressorFactory creates a Compressor from config.
type CompressorFactory func(config.CompressConfig) (compress.Compressor, error)

// UploaderFactory creates an Uploader from config.
type UploaderFactory func(config.UploadConfig) (upload.Uploader, error)

// NotifierFactory creates a Notifier from config.
type NotifierFactory func(config.NotifyConfig) (notify.Notifier, error)

// Scheduler manages cron-based backup jobs.
type Scheduler struct {
	cfg               *config.Config
	cron              *cron.Cron
	log               *slog.Logger
	dumperFactory     DumperFactory
	compressorFactory CompressorFactory
	uploaderFactory   UploaderFactory
	notifierFactory   NotifierFactory
	sem               chan struct{} // concurrency limiter
	wg                sync.WaitGroup
	maxConcurrent     int
}

// New creates a new Scheduler with the given config and logger.
func New(cfg *config.Config, log *slog.Logger) *Scheduler {
	if log == nil {
		log = slog.Default()
	}
	maxConcurrent := 2
	return &Scheduler{
		cfg:               cfg,
		cron:              cron.New(cron.WithSeconds()),
		log:               log,
		dumperFactory:     dump.NewDumper,
		compressorFactory: compress.NewCompressor,
		uploaderFactory:   upload.NewUploader,
		notifierFactory:   notify.NewNotifier,
		sem:               make(chan struct{}, maxConcurrent),
		maxConcurrent:     maxConcurrent,
	}
}

// WithDumperFactory sets a custom dumper factory (for testing).
func (s *Scheduler) WithDumperFactory(f DumperFactory) *Scheduler {
	s.dumperFactory = f
	return s
}

// WithCompressorFactory sets a custom compressor factory (for testing).
func (s *Scheduler) WithCompressorFactory(f CompressorFactory) *Scheduler {
	s.compressorFactory = f
	return s
}

// WithUploaderFactory sets a custom uploader factory (for testing).
func (s *Scheduler) WithUploaderFactory(f UploaderFactory) *Scheduler {
	s.uploaderFactory = f
	return s
}

// WithNotifierFactory sets a custom notifier factory (for testing).
func (s *Scheduler) WithNotifierFactory(f NotifierFactory) *Scheduler {
	s.notifierFactory = f
	return s
}

// WithMaxConcurrent sets the maximum number of concurrent backup jobs.
func (s *Scheduler) WithMaxConcurrent(n int) *Scheduler {
	if n < 1 {
		n = 1
	}
	s.maxConcurrent = n
	s.sem = make(chan struct{}, n)
	return s
}

// Start begins scheduling backup jobs and blocks until ctx is cancelled.
// On shutdown, waits for in-progress backups to finish (with a 60s timeout).
func (s *Scheduler) Start(ctx context.Context) error {
	if err := s.cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	for _, backupCfg := range s.cfg.Backups {
		if err := s.scheduleBackup(ctx, backupCfg); err != nil {
			return fmt.Errorf("failed to schedule backup %s: %w", backupCfg.Name, err)
		}
	}

	s.cron.Start()
	s.log.Info("scheduler started", "jobs", len(s.cfg.Backups), "max_concurrent", s.maxConcurrent)

	// Wait for shutdown signal
	<-ctx.Done()
	s.log.Info("stopping scheduler, waiting for in-progress backups...")
	s.cron.Stop()

	// Graceful drain: wait for in-progress backups with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.log.Info("all backups completed, shutdown clean")
	case <-time.After(60 * time.Second):
		s.log.Warn("shutdown timeout reached, some backups may not have completed")
	}

	return nil
}

func (s *Scheduler) scheduleBackup(ctx context.Context, cfg config.BackupConfig) error {
	_, err := s.cron.AddFunc(cfg.Schedule, func() {
		// Acquire semaphore slot
		s.sem <- struct{}{}
		s.wg.Add(1)
		defer func() {
			<-s.sem
			s.wg.Done()
		}()

		if err := s.runBackup(ctx, cfg); err != nil {
			s.log.Error("backup failed", "backup", cfg.Name, "error", err)
			s.notifyFailure(cfg, err)
		}
	})
	return err
}

// RunBackup executes a single backup job. Exported for testing.
func (s *Scheduler) RunBackup(ctx context.Context, cfg config.BackupConfig) error {
	return s.runBackup(ctx, cfg)
}

func (s *Scheduler) runBackup(ctx context.Context, cfg config.BackupConfig) error {
	s.log.Info("starting backup", "backup", cfg.Name)
	startTime := time.Now()

	// Step 1: Dump database
	dumper, err := s.dumperFactory(cfg.Database)
	if err != nil {
		return fmt.Errorf("create dumper: %w", err)
	}

	dumpFile, err := dumper.Dump(ctx)
	if err != nil {
		return fmt.Errorf("dump failed: %w", err)
	}
	defer os.Remove(dumpFile)

	// Step 2: Compress
	compressor, err := s.compressorFactory(cfg.Compress)
	if err != nil {
		return fmt.Errorf("create compressor: %w", err)
	}

	compressedFile, err := compressor.Compress(dumpFile)
	if err != nil {
		return fmt.Errorf("compress failed: %w", err)
	}
	if compressedFile != dumpFile {
		defer os.Remove(compressedFile)
	}

	// Step 3: Upload
	uploader, err := s.uploaderFactory(cfg.Upload)
	if err != nil {
		return fmt.Errorf("create uploader: %w", err)
	}

	remotePath, err := uploader.Upload(ctx, compressedFile, cfg.Name)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	duration := time.Since(startTime)
	s.log.Info("backup completed", "backup", cfg.Name, "duration", duration, "path", remotePath)

	// Step 4: Notify success
	s.notifySuccess(cfg, remotePath, duration)

	return nil
}

func (s *Scheduler) notifySuccess(cfg config.BackupConfig, path string, duration time.Duration) {
	if cfg.Notify.Type == "" || cfg.Notify.Type == "none" {
		return
	}

	notifier, notifyErr := s.notifierFactory(cfg.Notify)
	if notifyErr != nil {
		s.log.Error("failed to create notifier", "backup", cfg.Name, "error", notifyErr)
		return
	}

	msg := fmt.Sprintf("✅ Backup '%s' completed successfully\nPath: %s\nDuration: %v",
		cfg.Name, path, duration)
	if notifyErr := notifier.Notify(msg); notifyErr != nil {
		s.log.Error("failed to send success notification", "backup", cfg.Name, "error", notifyErr)
	}
}

func (s *Scheduler) notifyFailure(cfg config.BackupConfig, backupErr error) {
	if cfg.Notify.Type == "" || cfg.Notify.Type == "none" {
		return
	}

	notifier, notifyErr := s.notifierFactory(cfg.Notify)
	if notifyErr != nil {
		s.log.Error("failed to create notifier", "backup", cfg.Name, "error", notifyErr)
		return
	}

	msg := fmt.Sprintf("❌ Backup '%s' failed: %v", cfg.Name, backupErr)
	if notifyErr := notifier.Notify(msg); notifyErr != nil {
		s.log.Error("failed to send failure notification", "backup", cfg.Name, "error", notifyErr)
	}
}
