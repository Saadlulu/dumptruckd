package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Saadlulu/dumptruckd/internal/retry"
	"github.com/Saadlulu/dumptruckd/internal/utils"
	"github.com/Saadlulu/dumptruckd/pkg/compress"
	"github.com/Saadlulu/dumptruckd/pkg/config"
	"github.com/Saadlulu/dumptruckd/pkg/dump"
	"github.com/Saadlulu/dumptruckd/pkg/encrypt"
	"github.com/Saadlulu/dumptruckd/pkg/hooks"
	"github.com/Saadlulu/dumptruckd/pkg/notify"
	"github.com/Saadlulu/dumptruckd/pkg/retention"
	"github.com/Saadlulu/dumptruckd/pkg/sizetrack"
	"github.com/Saadlulu/dumptruckd/pkg/upload"
	"github.com/Saadlulu/dumptruckd/pkg/verify"
	"github.com/Saadlulu/dumptruckd/pkg/watchdog"
	"github.com/robfig/cron/v3"
)

// HealthRecorder is the interface for recording backup results to the health server.
type HealthRecorder interface {
	RecordSuccess(name string, duration time.Duration)
	RecordFailure(name string, err error)
	RecordSize(name string, bytes int64)
}

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
	sem               chan struct{}
	wg                sync.WaitGroup
	maxConcurrent     int
	watchdog          *watchdog.Watchdog
	health            HealthRecorder
	retryCfg          retry.Config
	shutdownTimeout   time.Duration
	jobLocks          map[string]*sync.Mutex     // per-job mutex to prevent duplicate runs
	jobLocksMu        sync.Mutex                 // protects jobLocks map
	uploaderCache     map[string]upload.Uploader // reuse uploaders (e.g. S3 sessions)
	uploaderCacheMu   sync.Mutex
	sizeTracker       *sizetrack.Tracker
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
		retryCfg:          retry.DefaultConfig(),
		shutdownTimeout:   60 * time.Second,
		jobLocks:          make(map[string]*sync.Mutex),
		uploaderCache:     make(map[string]upload.Uploader),
		sizeTracker:       sizetrack.NewTracker(),
	}
}

func (s *Scheduler) WithDumperFactory(f DumperFactory) *Scheduler {
	s.dumperFactory = f
	return s
}

func (s *Scheduler) WithCompressorFactory(f CompressorFactory) *Scheduler {
	s.compressorFactory = f
	return s
}

func (s *Scheduler) WithUploaderFactory(f UploaderFactory) *Scheduler {
	s.uploaderFactory = f
	return s
}

func (s *Scheduler) WithNotifierFactory(f NotifierFactory) *Scheduler {
	s.notifierFactory = f
	return s
}

func (s *Scheduler) WithMaxConcurrent(n int) *Scheduler {
	if n < 1 {
		n = 1
	}
	s.maxConcurrent = n
	s.sem = make(chan struct{}, n)
	return s
}

func (s *Scheduler) WithWatchdog(w *watchdog.Watchdog) *Scheduler {
	s.watchdog = w
	return s
}

func (s *Scheduler) WithHealthRecorder(h HealthRecorder) *Scheduler {
	s.health = h
	return s
}

func (s *Scheduler) WithRetryConfig(cfg retry.Config) *Scheduler {
	s.retryCfg = cfg
	return s
}

func (s *Scheduler) WithShutdownTimeout(d time.Duration) *Scheduler {
	s.shutdownTimeout = d
	return s
}

// WithSizeTracker sets a custom size tracker for the scheduler.
func (s *Scheduler) WithSizeTracker(t *sizetrack.Tracker) *Scheduler {
	s.sizeTracker = t
	return s
}

// Start begins scheduling backup jobs and blocks until ctx is cancelled.
// On shutdown, waits for in-progress backups to finish (with a 60s timeout).
func (s *Scheduler) Start(ctx context.Context) error {
	if err := s.cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Pre-flight check: verify all adapters can be created now, not at 2am
	for _, backupCfg := range s.cfg.Backups {
		if _, err := s.dumperFactory(backupCfg.Database); err != nil {
			return fmt.Errorf("backup %s: %w (check env vars and config)", backupCfg.Name, err)
		}
		if _, err := s.compressorFactory(backupCfg.Compress); err != nil {
			return fmt.Errorf("backup %s: %w", backupCfg.Name, err)
		}
		if _, err := s.uploaderFactory(backupCfg.Upload); err != nil {
			return fmt.Errorf("backup %s: %w (check env vars and config)", backupCfg.Name, err)
		}
	}

	cronParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	for _, backupCfg := range s.cfg.Backups {
		if err := s.scheduleBackup(ctx, backupCfg); err != nil {
			return fmt.Errorf("failed to schedule backup %s: %w", backupCfg.Name, err)
		}

		// Register with watchdog if available
		if s.watchdog != nil {
			interval := estimateCronInterval(s.log, cronParser, backupCfg.Schedule)
			s.watchdog.Register(backupCfg.Name, interval)
		}
	}

	s.cron.Start()
	s.log.Info("scheduler started", "jobs", len(s.cfg.Backups), "max_concurrent", s.maxConcurrent)

	// Start watchdog periodic checks (every 5 minutes)
	if s.watchdog != nil {
		s.watchdog.StartPeriodicCheck(5 * time.Minute)
	}

	// Wait for shutdown signal
	<-ctx.Done()
	s.log.Info("stopping scheduler, waiting for in-progress backups...")
	s.cron.Stop()

	if s.watchdog != nil {
		s.watchdog.Stop()
	}

	// Graceful drain: wait for in-progress backups with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.log.Info("all backups completed, shutdown clean")
	case <-time.After(s.shutdownTimeout):
		s.log.Warn("shutdown timeout reached, some backups may not have completed",
			"timeout", s.shutdownTimeout)
	}

	return nil
}

// estimateCronInterval calculates the maximum interval between cron runs.
// Samples multiple consecutive runs to handle irregular schedules (e.g. weekday-only
// schedules where the Friday→Monday gap is larger than the typical daily gap).
func estimateCronInterval(log *slog.Logger, parser cron.Parser, schedule string) time.Duration {
	sched, err := parser.Parse(schedule)
	if err != nil {
		log.Warn("failed to parse cron schedule for watchdog interval, falling back to 24h",
			"schedule", schedule, "error", err)
		return 24 * time.Hour // fallback to daily
	}

	// Sample 8 consecutive intervals to find the maximum gap.
	// This covers a full week for daily schedules and catches weekend gaps.
	now := utils.Now()
	maxInterval := time.Duration(0)
	prev := sched.Next(now)
	for i := 0; i < 8; i++ {
		next := sched.Next(prev)
		gap := next.Sub(prev)
		if gap > maxInterval {
			maxInterval = gap
		}
		prev = next
	}

	// Clamp to reasonable bounds
	if maxInterval < 1*time.Minute {
		maxInterval = 1 * time.Minute
	}
	if maxInterval > 7*24*time.Hour {
		maxInterval = 7 * 24 * time.Hour
	}

	return maxInterval
}

func (s *Scheduler) scheduleBackup(ctx context.Context, cfg config.BackupConfig) error {
	_, err := s.cron.AddFunc(cfg.Schedule, func() {
		// Track intent before blocking on semaphore so wg.Wait() during
		// shutdown doesn't return early while goroutines are queued.
		s.wg.Add(1)
		defer s.wg.Done()

		// Per-job lock: skip if this job is already running
		mu := s.getJobLock(cfg.Name)
		if !mu.TryLock() {
			s.log.Warn("backup already running, skipping", "backup", cfg.Name)
			return
		}
		defer mu.Unlock()

		// Acquire semaphore slot
		s.sem <- struct{}{}
		defer func() { <-s.sem }()

		if err := s.runBackup(ctx, cfg); err != nil {
			s.log.Error("backup failed", "backup", cfg.Name, "error", err)
			if s.watchdog != nil {
				s.watchdog.RecordFailure(cfg.Name)
			}
			if s.health != nil {
				s.health.RecordFailure(cfg.Name, err)
			}
			s.notifyFailure(ctx, cfg, err)
		}
	})
	return err
}

// getJobLock returns the mutex for a given backup job name, creating it if needed.
func (s *Scheduler) getJobLock(name string) *sync.Mutex {
	s.jobLocksMu.Lock()
	defer s.jobLocksMu.Unlock()
	mu, ok := s.jobLocks[name]
	if !ok {
		mu = &sync.Mutex{}
		s.jobLocks[name] = mu
	}
	return mu
}

// RunBackup executes a single backup job. Exported for testing.
func (s *Scheduler) RunBackup(ctx context.Context, cfg config.BackupConfig) error {
	return s.runBackup(ctx, cfg)
}

// ValidateAdapters verifies that all adapters for a backup can be created.
// Used by --dry-run to catch configuration errors without executing backups.
func (s *Scheduler) ValidateAdapters(cfg config.BackupConfig) error {
	if _, err := s.dumperFactory(cfg.Database); err != nil {
		return fmt.Errorf("dumper: %w", err)
	}
	if _, err := s.compressorFactory(cfg.Compress); err != nil {
		return fmt.Errorf("compressor: %w", err)
	}
	if _, err := s.uploaderFactory(cfg.Upload); err != nil {
		return fmt.Errorf("uploader: %w", err)
	}
	return nil
}

func (s *Scheduler) runBackup(ctx context.Context, cfg config.BackupConfig) error {
	s.log.Info(fmt.Sprintf("=== backup: %s ===", cfg.Name))
	startTime := utils.Now()

	// Count total steps for the progress prefix
	totalSteps := 3 // dump + compress + upload (always present)
	hasEncrypt := cfg.Encrypt.Type != "" && cfg.Encrypt.Type != "none"
	if hasEncrypt {
		totalSteps++
	}
	if cfg.Verify {
		totalSteps++
	}
	step := 0
	nextStep := func() int { step++; return step }

	// Build hook environment map
	hookEnv := map[string]string{
		"DUMPTRUCKD_HOOK_BACKUP_NAME": cfg.Name,
		"DUMPTRUCKD_HOOK_STATUS":      "pending",
		"DUMPTRUCKD_HOOK_FILE_PATH":   "",
	}

	// Pre-hook
	if cfg.Hooks.Pre != "" {
		s.log.Info("[hook] running pre-backup hook...")
		if err := hooks.RunHook(ctx, cfg.Hooks.Pre, hookEnv, hooks.DefaultTimeout); err != nil {
			s.log.Error("[hook] pre-hook failed, aborting", "error", err)
			s.runPostHook(ctx, cfg, hookEnv, "failure", "")
			return fmt.Errorf("pre-hook failed: %w", err)
		}
	}

	// Step 1: Dump
	n := nextStep()
	dumper, err := s.dumperFactory(cfg.Database)
	if err != nil {
		s.runPostHook(ctx, cfg, hookEnv, "failure", "")
		return fmt.Errorf("create dumper: %w", err)
	}

	var dumpFile string
	err = retry.Do(ctx, s.retryCfg, s.log, "database dump", func() error {
		var dumpErr error
		dumpFile, dumpErr = dumper.Dump(ctx)
		if dumpErr != nil && dumpFile != "" {
			os.Remove(dumpFile)
			dumpFile = ""
		}
		return dumpErr
	})
	if err != nil {
		s.runPostHook(ctx, cfg, hookEnv, "failure", "")
		return fmt.Errorf("dump failed: %w", err)
	}
	defer os.Remove(dumpFile)

	dumpSize := fileSize(dumpFile)
	_ = n // dump step logged by the dumper itself

	// Step 2: Compress
	n = nextStep()
	compressor, err := s.compressorFactory(cfg.Compress)
	if err != nil {
		s.runPostHook(ctx, cfg, hookEnv, "failure", "")
		return fmt.Errorf("create compressor: %w", err)
	}

	if cfg.Compress.Type != "none" && cfg.Compress.Type != "" {
		s.log.Info(fmt.Sprintf("[%d/%d] compressing %s with %s...", n, totalSteps, formatSize(dumpSize), cfg.Compress.Type))
	}

	compressStart := utils.Now()
	compressedFile, err := compressor.Compress(ctx, dumpFile)
	if err != nil {
		s.runPostHook(ctx, cfg, hookEnv, "failure", "")
		return fmt.Errorf("compress failed: %w", err)
	}
	if compressedFile != dumpFile {
		defer os.Remove(compressedFile)
		compressedSize := fileSize(compressedFile)
		compressDur := utils.Now().Sub(compressStart).Round(time.Second)
		if dumpSize > 0 {
			reduction := 100 - (float64(compressedSize) / float64(dumpSize) * 100)
			s.log.Info(fmt.Sprintf("[%d/%d] compressed: %s -> %s (%.0f%% reduction, %s)", n, totalSteps, formatSize(dumpSize), formatSize(compressedSize), reduction, compressDur))
		} else {
			s.log.Info(fmt.Sprintf("[%d/%d] compressed: %s (%s)", n, totalSteps, formatSize(compressedSize), compressDur))
		}
	}

	// Step 3: Encrypt (optional)
	fileToUpload := compressedFile
	if hasEncrypt {
		n = nextStep()
		s.log.Info(fmt.Sprintf("[%d/%d] encrypting with %s...", n, totalSteps, cfg.Encrypt.Type))

		encryptor, encErr := encrypt.NewEncryptor(cfg.Encrypt)
		if encErr != nil {
			s.runPostHook(ctx, cfg, hookEnv, "failure", "")
			return fmt.Errorf("create encryptor: %w", encErr)
		}

		encryptedFile, encErr := encryptor.Encrypt(ctx, compressedFile)
		if encErr != nil {
			s.runPostHook(ctx, cfg, hookEnv, "failure", "")
			return fmt.Errorf("encrypt failed: %w", encErr)
		}
		if encryptedFile != compressedFile {
			defer os.Remove(encryptedFile)
		}
		fileToUpload = encryptedFile
		s.log.Info(fmt.Sprintf("[%d/%d] encrypted: %s", n, totalSteps, formatSize(fileSize(fileToUpload))))
	}

	// Step 4: Upload
	n = nextStep()
	s.log.Info(fmt.Sprintf("[%d/%d] uploading %s to %s...", n, totalSteps, formatSize(fileSize(fileToUpload)), cfg.Upload.Type))

	uploader, err := s.getOrCreateUploader(cfg.Upload)
	if err != nil {
		s.runPostHook(ctx, cfg, hookEnv, "failure", "")
		return fmt.Errorf("create uploader: %w", err)
	}

	uploadStart := utils.Now()
	var remotePath string
	err = retry.Do(ctx, s.retryCfg, s.log, "upload", func() error {
		var uploadErr error
		remotePath, uploadErr = uploader.Upload(ctx, fileToUpload, cfg.Name)
		return uploadErr
	})
	if err != nil {
		s.runPostHook(ctx, cfg, hookEnv, "failure", "")
		return fmt.Errorf("upload failed: %w", err)
	}
	uploadDur := utils.Now().Sub(uploadStart).Round(time.Second)
	s.log.Info(fmt.Sprintf("[%d/%d] uploaded: %s (%s)", n, totalSteps, remotePath, uploadDur))

	duration := utils.Now().Sub(startTime)

	// Record success
	if s.watchdog != nil {
		s.watchdog.RecordSuccess(cfg.Name)
	}
	if s.health != nil {
		s.health.RecordSuccess(cfg.Name, duration)
	}

	// Step 5: Verify (optional)
	if cfg.Verify {
		n = nextStep()
		s.log.Info(fmt.Sprintf("[%d/%d] verifying backup...", n, totalSteps))
		s.runVerify(ctx, cfg, uploader, remotePath)
	}

	// Size tracking
	s.runSizeTrack(ctx, cfg, fileToUpload, remotePath)

	// Post-hook
	s.runPostHook(ctx, cfg, hookEnv, "success", remotePath)

	// Notify
	s.notifySuccess(ctx, cfg, remotePath, duration)

	// Retention
	if cfg.Retention.Days > 0 || cfg.Retention.KeepLast > 0 {
		s.log.Info("[cleanup] running retention...")
	}
	s.runRetention(cfg)

	s.log.Info(fmt.Sprintf("=== backup complete: %s (%s) ===", cfg.Name, duration.Round(time.Second)))
	return nil
}

// runPostHook executes the post-hook command if configured. Failure is logged
// as a warning but does not affect backup status (Req 11.4).
func (s *Scheduler) runPostHook(ctx context.Context, cfg config.BackupConfig, env map[string]string, status string, filePath string) {
	if cfg.Hooks.Post == "" {
		return
	}
	env["DUMPTRUCKD_HOOK_STATUS"] = status
	env["DUMPTRUCKD_HOOK_FILE_PATH"] = filePath
	s.log.Info("running post-hook", "backup", cfg.Name, "status", status)
	if err := hooks.RunHook(ctx, cfg.Hooks.Post, env, hooks.DefaultTimeout); err != nil {
		s.log.Warn("post-hook failed", "backup", cfg.Name, "error", err)
	}
}

// runVerify performs post-upload backup verification. Failure logs an error and
// sends a notification but does NOT mark the backup as failed (Req 7.4).
func (s *Scheduler) runVerify(ctx context.Context, cfg config.BackupConfig, uploader upload.Uploader, remotePath string) {
	var downloader verify.Downloader
	if cfg.Upload.Type == "local" {
		downloader = verify.NewLocalDownloader()
	} else {
		s.log.Warn("verification not supported for upload type, skipping", "backup", cfg.Name, "type", cfg.Upload.Type)
		return
	}

	verifier := verify.NewVerifier(downloader, s.log)
	if err := verifier.Verify(ctx, remotePath, cfg.Database.Type, cfg.Compress.Type, cfg.Encrypt.Type); err != nil {
		s.log.Error("backup verification failed", "backup", cfg.Name, "path", remotePath, "error", err)
		s.sendNotification(ctx, cfg, fmt.Sprintf("Backup '%s' verification failed: %v\nPath: %s", cfg.Name, err, remotePath))
	}
}

// runSizeTrack records the backup file size and sends an alert if an anomaly is detected (Req 9.3).
func (s *Scheduler) runSizeTrack(ctx context.Context, cfg config.BackupConfig, localFile string, remotePath string) {
	if s.sizeTracker == nil {
		return
	}

	info, err := os.Stat(localFile)
	if err != nil {
		s.log.Warn("failed to stat backup file for size tracking", "backup", cfg.Name, "error", err)
		return
	}

	sizeBytes := info.Size()

	// Set custom threshold if configured
	if cfg.SizeAlertThreshold > 0 {
		s.sizeTracker.SetThreshold(cfg.Name, cfg.SizeAlertThreshold)
	}

	anomaly := s.sizeTracker.Record(cfg.Name, sizeBytes)

	// Record size in health server (Req 13.1)
	if s.health != nil {
		s.health.RecordSize(cfg.Name, sizeBytes)
	}

	if anomaly != nil {
		msg := fmt.Sprintf("Backup '%s' size anomaly detected\nCurrent: %d bytes\nRolling avg: %d bytes\nDeviation: %.1f%%\nPath: %s",
			cfg.Name, anomaly.CurrentSize, anomaly.RollingAvg, anomaly.DeviationPct, remotePath)
		s.log.Warn("backup size anomaly", "backup", cfg.Name,
			"current_size", anomaly.CurrentSize, "rolling_avg", anomaly.RollingAvg,
			"deviation_pct", anomaly.DeviationPct)
		s.sendNotification(ctx, cfg, msg)
	}
}

// sendNotification sends a message through the configured notifier.
func (s *Scheduler) sendNotification(ctx context.Context, cfg config.BackupConfig, msg string) {
	if cfg.Notify.Type == "" || cfg.Notify.Type == "none" {
		return
	}

	notifier, notifyErr := s.notifierFactory(cfg.Notify)
	if notifyErr != nil {
		s.log.Error("failed to create notifier", "backup", cfg.Name, "error", notifyErr)
		return
	}

	notifyCfg := retry.Config{MaxRetries: 2, BaseDelay: 1 * time.Second}
	if retryErr := retry.Do(ctx, notifyCfg, s.log, "notification", func() error {
		return notifier.Notify(ctx, msg)
	}); retryErr != nil {
		s.log.Error("failed to send notification", "backup", cfg.Name, "error", retryErr)
	}
}

// getOrCreateUploader returns a cached uploader for the given config, creating one if needed.
// This avoids recreating AWS sessions on every backup run.
func (s *Scheduler) getOrCreateUploader(cfg config.UploadConfig) (upload.Uploader, error) {
	key := fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		cfg.Type, cfg.S3.Bucket, cfg.S3.Region, cfg.S3.Endpoint, cfg.S3.Prefix, cfg.Path)
	s.uploaderCacheMu.Lock()
	defer s.uploaderCacheMu.Unlock()
	if u, ok := s.uploaderCache[key]; ok {
		return u, nil
	}
	u, err := s.uploaderFactory(cfg)
	if err != nil {
		return nil, err
	}
	s.uploaderCache[key] = u
	return u, nil
}

// runRetention performs local filesystem retention cleanup if configured.
func (s *Scheduler) runRetention(cfg config.BackupConfig) {
	if cfg.Retention.Days <= 0 && cfg.Retention.KeepLast <= 0 {
		return
	}
	if cfg.Upload.Type != "local" {
		return // S3 retention is handled by lifecycle policies
	}

	basePath := cfg.Upload.Path
	if basePath == "" {
		basePath = "/var/backups/dumptruckd"
	}

	// Scope retention to this backup's subdirectory to avoid deleting
	// files belonging to other backup jobs that share the same base path.
	scopedPath := filepath.Join(basePath, cfg.Name)

	r := retention.New(scopedPath, cfg.Retention.Days, cfg.Retention.KeepLast)
	if err := r.Cleanup(); err != nil {
		s.log.Error("retention cleanup failed", "backup", cfg.Name, "path", scopedPath, "error", err)
	} else {
		s.log.Debug("retention cleanup completed", "backup", cfg.Name, "days", cfg.Retention.Days, "keep_last", cfg.Retention.KeepLast)
	}
}

func (s *Scheduler) notifySuccess(ctx context.Context, cfg config.BackupConfig, path string, duration time.Duration) {
	msg := fmt.Sprintf("Backup '%s' completed successfully\nPath: %s\nDuration: %v",
		cfg.Name, path, duration)
	s.sendNotification(ctx, cfg, msg)
}

func (s *Scheduler) notifyFailure(ctx context.Context, cfg config.BackupConfig, backupErr error) {
	msg := fmt.Sprintf("Backup '%s' failed: %v", cfg.Name, backupErr)
	s.sendNotification(ctx, cfg, msg)
}

// fileSize returns the size of a file in bytes, or 0 if the file can't be stat'd.
func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// formatSize returns a human-readable byte size.
func formatSize(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
