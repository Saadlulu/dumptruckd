package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pelletier/go-toml/v2"
	"github.com/robfig/cron/v3"

	"github.com/Saadlulu/dumptruckd/internal/logger"
	"github.com/Saadlulu/dumptruckd/internal/retry"
	"github.com/Saadlulu/dumptruckd/pkg/config"
	"github.com/Saadlulu/dumptruckd/pkg/health"
	"github.com/Saadlulu/dumptruckd/pkg/notify"
	"github.com/Saadlulu/dumptruckd/pkg/restore"
	"github.com/Saadlulu/dumptruckd/pkg/scheduler"
	"github.com/Saadlulu/dumptruckd/pkg/verify"
	"github.com/Saadlulu/dumptruckd/pkg/watchdog"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Handle "restore" subcommand before flag.Parse() so it gets its own FlagSet.
	if len(os.Args) > 1 && os.Args[1] == "restore" {
		runRestoreSubcommand()
		return
	}

	var (
		configPath  = flag.String("config", "", "Path to configuration file")
		versionFlag = flag.Bool("version", false, "Print version and exit")
		testFlag    = flag.Bool("test", false, "Run configuration tests and exit")
		runNowFlag  = flag.Bool("run-now", false, "Run all backups immediately and exit")
		dryRunFlag  = flag.Bool("dry-run", false, "Validate config and adapters without executing backups")
		onceFlag    = flag.Bool("once", false, "Run all backups once and exit (no scheduler)")
		verboseFlag = flag.Bool("verbose", false, "Show detailed output (used with -test)")
		dumpCfgFlag = flag.Bool("dump-config", false, "Load, merge, and print the final resolved config as TOML, then exit")
	)
	flag.Parse()

	if *versionFlag {
		fmt.Printf("dumptruckd version %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Find config file
	if *configPath == "" {
		*configPath = findConfig()
	}

	if *testFlag {
		runTest(*configPath, *verboseFlag)
		return
	}

	// Load configuration: file path -> env vars fallback
	cfg, err := loadConfig(*configPath)
	if err != nil {
		printConfigUsage()
		log.Fatalf("Failed to load config: %v", err)
	}

	// --dump-config: print the final resolved config and exit
	if *dumpCfgFlag {
		data, marshalErr := toml.Marshal(cfg)
		if marshalErr != nil {
			log.Fatalf("Failed to marshal config: %v", marshalErr)
		}
		fmt.Print(string(data))
		return
	}

	slogger, err := logger.New(cfg.Logging)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer slogger.Close()

	// --dry-run: validate config and adapters, then exit
	if *dryRunFlag {
		runDryRun(cfg, slogger.Logger)
		return
	}

	// --once: run all backups once sequentially and exit (minimal path, no scheduler)
	if *onceFlag {
		runOnce(cfg, slogger.Logger)
		return
	}

	// --run-now: execute all backups immediately and exit
	if *runNowFlag {
		runAllNow(cfg, slogger.Logger)
		return
	}

	var healthServer *health.Server
	if cfg.Health.Enabled {
		healthServer = health.New(cfg.Health.Port, slogger.Logger).WithToken(cfg.Health.Token)
		if err := healthServer.Start(); err != nil {
			log.Fatalf("Failed to start health server: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		slogger.Logger.Info("shutting down gracefully...")
		cancel()
	}()

	alerter := buildAlerter(ctx, cfg, slogger.Logger)
	wd := watchdog.New(slogger.Logger, alerter)

	sched := scheduler.New(cfg, slogger.Logger).WithWatchdog(wd)
	if healthServer != nil {
		sched = sched.WithHealthRecorder(healthServer)
	}

	if err := sched.Start(ctx); err != nil {
		log.Fatalf("Scheduler error: %v", err)
	}

	if healthServer != nil {
		if err := healthServer.Stop(context.Background()); err != nil {
			slogger.Logger.Error("failed to stop health server", "error", err)
		}
	}

	slogger.Logger.Info("dumptruckd stopped")
}

// findConfig looks for config in common locations.
// Returns the path to the first config file found, or "" if none exists.
func findConfig() string {
	candidates := []string{
		"/etc/dumptruckd/dumptruckd.toml",
		"config/dumptruckd.toml",
		"dumptruckd.toml",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// loadConfig loads configuration from a file path or falls back to environment
// variables. When configPath is non-empty, the file is loaded and env vars are
// ignored (Req 1.6). When configPath is empty, LoadFromEnv() is tried. If
// neither source provides a config, an error is returned.
func loadConfig(configPath string) (*config.Config, error) {
	if configPath != "" {
		return config.Load(configPath)
	}

	// No config file -- try environment variables.
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return nil, fmt.Errorf("env-var config: %w", err)
	}
	if cfg != nil {
		return cfg, nil
	}

	return nil, fmt.Errorf("no configuration found")
}

// printConfigUsage prints help text when no config source is found.
func printConfigUsage() {
	candidates := []string{
		"/etc/dumptruckd/dumptruckd.toml",
		"config/dumptruckd.toml",
		"dumptruckd.toml",
	}
	fmt.Fprintln(os.Stderr, "No config file found. Looked in:")
	for _, path := range candidates {
		fmt.Fprintf(os.Stderr, "  - %s\n", path)
	}
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Alternatively, configure via environment variables by setting")
	fmt.Fprintln(os.Stderr, "DUMPTRUCKD_DB_TYPE and other DUMPTRUCKD_* variables.")
	fmt.Fprintln(os.Stderr, "See docs/CONFIGURATION.md for the full list.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "To get started, run the interactive installer:")
	fmt.Fprintln(os.Stderr, "  sudo dumptruckd-install")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Or specify a config file:")
	fmt.Fprintln(os.Stderr, "  dumptruckd -config /path/to/dumptruckd.toml")
}

// runAllNow executes every backup job immediately and exits.
func runAllNow(cfg *config.Config, log *slog.Logger) {
	if err := cfg.Validate(); err != nil {
		log.Error("config validation failed", "error", err)
		os.Exit(1)
	}

	sched := scheduler.New(cfg, log).WithRetryConfig(retry.DefaultConfig())
	ctx := context.Background()
	failed := 0

	for _, backup := range cfg.Backups {
		log.Info("running backup now", "backup", backup.Name)
		if err := sched.RunBackup(ctx, backup); err != nil {
			log.Error("backup failed", "backup", backup.Name, "error", err)
			failed++
		}
	}

	if failed > 0 {
		log.Error("some backups failed", "failed", failed, "total", len(cfg.Backups))
		os.Exit(1)
	}

	log.Info("all backups completed successfully", "total", len(cfg.Backups))
}

// runDryRun validates config, verifies all adapters, checks S3 bucket accessibility,
// sends a test notification if configured, and prints the next 3 scheduled run times
// per backup job. Exits 0 on all pass, 1 on any failure. (Req 14.1–14.5)
func runDryRun(cfg *config.Config, log *slog.Logger) {
	if err := cfg.Validate(); err != nil {
		log.Error("config validation failed", "error", err)
		os.Exit(1)
	}
	log.Info("config validation passed", "backups", len(cfg.Backups))

	// Try creating all adapters to catch missing env vars, bad credentials, etc.
	sched := scheduler.New(cfg, log)
	failed := 0

	cronParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	for _, backup := range cfg.Backups {
		log.Info("dry-run: checking adapters", "backup", backup.Name)
		if err := sched.ValidateAdapters(backup); err != nil {
			log.Error("adapter creation failed", "backup", backup.Name, "error", err)
			failed++
		} else {
			log.Info("dry-run: adapters OK", "backup", backup.Name)
		}

		// S3 HeadBucket check (Req 14.1)
		if backup.Upload.Type == "s3" {
			if err := checkS3Bucket(backup.Upload.S3); err != nil {
				log.Error("dry-run: S3 bucket not accessible", "backup", backup.Name, "bucket", backup.Upload.S3.Bucket, "error", err)
				failed++
			} else {
				log.Info("dry-run: S3 bucket accessible", "backup", backup.Name, "bucket", backup.Upload.S3.Bucket)
			}
		}

		// Test notification (Req 14.2)
		if backup.Notify.Type != "" && backup.Notify.Type != "none" {
			notifier, err := notify.NewNotifier(backup.Notify)
			if err != nil {
				log.Error("dry-run: failed to create notifier", "backup", backup.Name, "error", err)
				failed++
			} else {
				ctx := context.Background()
				testMsg := fmt.Sprintf("dumptruckd dry-run test notification for backup '%s'", backup.Name)
				if err := notifier.Notify(ctx, testMsg); err != nil {
					log.Error("dry-run: test notification failed", "backup", backup.Name, "error", err)
					failed++
				} else {
					log.Info("dry-run: test notification sent", "backup", backup.Name, "type", backup.Notify.Type)
				}
			}
		}

		// Print next 3 scheduled run times (Req 14.3)
		schedule, err := cronParser.Parse(backup.Schedule)
		if err != nil {
			log.Error("dry-run: failed to parse schedule", "backup", backup.Name, "schedule", backup.Schedule, "error", err)
			failed++
		} else {
			now := time.Now()
			log.Info("dry-run: next scheduled runs", "backup", backup.Name)
			next := now
			for i := 0; i < 3; i++ {
				next = schedule.Next(next)
				log.Info("dry-run: scheduled run", "backup", backup.Name, "run", i+1, "time", next.Format(time.RFC3339))
			}
		}
	}

	if failed > 0 {
		log.Error("dry-run failed", "failed", failed, "total", len(cfg.Backups))
		os.Exit(1)
	}

	log.Info("dry-run passed — all configs, adapters, and connectivity checks are valid")
}

// checkS3Bucket performs a HeadBucket call to verify the S3 bucket is accessible.
func checkS3Bucket(s3Cfg config.S3Config) error {
	region := s3Cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	awsCfg := &aws.Config{
		Region: aws.String(region),
	}
	if s3Cfg.Endpoint != "" {
		awsCfg.Endpoint = aws.String(s3Cfg.Endpoint)
		awsCfg.S3ForcePathStyle = aws.Bool(true)
	}

	sess, err := session.NewSession(awsCfg)
	if err != nil {
		return fmt.Errorf("create AWS session: %w", err)
	}

	client := s3.New(sess)
	_, err = client.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(s3Cfg.Bucket),
	})
	if err != nil {
		return fmt.Errorf("HeadBucket failed: %w", err)
	}

	return nil
}

// runOnce executes all configured backups once sequentially and exits.
// Unlike --run-now, this is a minimal path: no scheduler initialization, no retry
// config, no watchdog, no health server. Designed for single-shot container
// execution (e.g., Kamal or cron). (Req 3.1–3.5)
func runOnce(cfg *config.Config, log *slog.Logger) {
	if err := cfg.Validate(); err != nil {
		log.Error("config validation failed", "error", err)
		os.Exit(1)
	}

	sched := scheduler.New(cfg, log).WithRetryConfig(retry.DefaultConfig())
	ctx := context.Background()
	failed := 0

	for _, backup := range cfg.Backups {
		log.Info("once: running backup", "backup", backup.Name)
		if err := sched.RunBackup(ctx, backup); err != nil {
			log.Error("once: backup failed", "backup", backup.Name, "error", err)
			failed++
		} else {
			log.Info("once: backup succeeded", "backup", backup.Name)
		}
	}

	if failed > 0 {
		log.Error("once: some backups failed", "failed", failed, "total", len(cfg.Backups))
		os.Exit(1)
	}

	log.Info("once: all backups completed successfully", "total", len(cfg.Backups))
}

// runRestoreSubcommand handles the "dumptruckd restore" subcommand with its own
// FlagSet. Parses --backup, --latest, --timestamp, and --config flags. (Req 6.1, 6.2)
func runRestoreSubcommand() {
	restoreCmd := flag.NewFlagSet("restore", flag.ExitOnError)
	backupName := restoreCmd.String("backup", "", "Backup name to restore")
	latest := restoreCmd.Bool("latest", false, "Restore the latest backup")
	timestamp := restoreCmd.String("timestamp", "", "Restore backup at specific timestamp")
	configPath := restoreCmd.String("config", "", "Path to configuration file")
	if err := restoreCmd.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *backupName == "" {
		fmt.Fprintln(os.Stderr, "Error: --backup is required")
		fmt.Fprintln(os.Stderr, "Usage: dumptruckd restore --backup <name> --latest [--config <path>]")
		fmt.Fprintln(os.Stderr, "       dumptruckd restore --backup <name> --timestamp <ts> [--config <path>]")
		os.Exit(1)
	}

	if !*latest && *timestamp == "" {
		fmt.Fprintln(os.Stderr, "Error: either --latest or --timestamp must be specified")
		fmt.Fprintln(os.Stderr, "Usage: dumptruckd restore --backup <name> --latest [--config <path>]")
		fmt.Fprintln(os.Stderr, "       dumptruckd restore --backup <name> --timestamp <ts> [--config <path>]")
		os.Exit(1)
	}

	// Resolve config path
	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = findConfig()
	}

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	slogger, err := logger.New(cfg.Logging)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer slogger.Close()

	// Find the backup config by name
	var backupCfg *config.BackupConfig
	for i := range cfg.Backups {
		if cfg.Backups[i].Name == *backupName {
			backupCfg = &cfg.Backups[i]
			break
		}
	}
	if backupCfg == nil {
		slogger.Logger.Error("backup not found in config", "backup", *backupName)
		os.Exit(1)
	}

	// Create the appropriate lister and downloader based on upload type
	var lister restore.Lister
	var downloader verify.Downloader

	switch backupCfg.Upload.Type {
	case "local":
		basePath := backupCfg.Upload.Path
		if basePath == "" {
			basePath = "/var/backups/dumptruckd"
		}
		lister = restore.NewLocalLister(basePath)
		downloader = verify.NewLocalDownloader()
	default:
		slogger.Logger.Error("restore not yet supported for upload type", "type", backupCfg.Upload.Type)
		os.Exit(1)
	}

	restorer := restore.NewRestorer(*backupCfg, downloader, lister, slogger.Logger)

	opts := restore.RestoreOpts{
		BackupName: *backupName,
		Latest:     *latest,
		Timestamp:  *timestamp,
	}

	ctx := context.Background()
	if err := restorer.Restore(ctx, opts); err != nil {
		slogger.Logger.Error("restore failed", "backup", *backupName, "error", err)
		os.Exit(1)
	}

	slogger.Logger.Info("restore completed successfully", "backup", *backupName)
}

// buildAlerter creates a watchdog alerter that fans out to all configured notifiers.
// If multiple backups have different notification configs, all are used.
func buildAlerter(ctx context.Context, cfg *config.Config, log *slog.Logger) watchdog.Alerter {
	var notifiers []notify.Notifier
	seen := make(map[string]bool)

	for _, backup := range cfg.Backups {
		if backup.Notify.Type == "" || backup.Notify.Type == "none" {
			continue
		}
		// Deduplicate by type+config key
		key := fmt.Sprintf("%s:%s:%s", backup.Notify.Type, backup.Notify.Slack.WebhookURL, backup.Notify.Webhook.URL)
		if seen[key] {
			continue
		}
		seen[key] = true

		notifier, err := notify.NewNotifier(backup.Notify)
		if err == nil {
			notifiers = append(notifiers, notifier)
		} else {
			log.Warn("watchdog: failed to create notifier, alerts may be lost",
				"backup", backup.Name, "type", backup.Notify.Type, "error", err)
		}
	}

	if len(notifiers) == 1 {
		return watchdog.NewNotifyAlerter(notifiers[0]).WithContext(ctx)
	}
	if len(notifiers) > 1 {
		return watchdog.NewMultiAlerter(notifiers, log, ctx)
	}
	return watchdog.NewLogAlerter(log)
}
