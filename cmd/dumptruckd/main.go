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

	"github.com/Saadlulu/dumptruckd/internal/logger"
	"github.com/Saadlulu/dumptruckd/internal/retry"
	"github.com/Saadlulu/dumptruckd/pkg/config"
	"github.com/Saadlulu/dumptruckd/pkg/health"
	"github.com/Saadlulu/dumptruckd/pkg/notify"
	"github.com/Saadlulu/dumptruckd/pkg/scheduler"
	"github.com/Saadlulu/dumptruckd/pkg/watchdog"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	var (
		configPath  = flag.String("config", "", "Path to configuration file")
		versionFlag = flag.Bool("version", false, "Print version and exit")
		testFlag    = flag.Bool("test", false, "Run configuration tests and exit")
		runNowFlag  = flag.Bool("run-now", false, "Run all backups immediately and exit")
		dryRunFlag  = flag.Bool("dry-run", false, "Validate config and adapters without executing backups")
		verboseFlag = flag.Bool("verbose", false, "Show detailed output (used with -test)")
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

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
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

	fmt.Fprintln(os.Stderr, "No config file found. Looked in:")
	for _, path := range candidates {
		fmt.Fprintf(os.Stderr, "  - %s\n", path)
	}
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "To get started, run the interactive installer:")
	fmt.Fprintln(os.Stderr, "  sudo dumptruckd-install")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Or specify a config file:")
	fmt.Fprintln(os.Stderr, "  dumptruckd -config /path/to/dumptruckd.toml")
	os.Exit(1)
	return ""
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

// runDryRun validates config and verifies all adapters can be created, without
// executing any actual backups. Useful for CI/CD pipeline validation.
func runDryRun(cfg *config.Config, log *slog.Logger) {
	if err := cfg.Validate(); err != nil {
		log.Error("config validation failed", "error", err)
		os.Exit(1)
	}
	log.Info("config validation passed", "backups", len(cfg.Backups))

	// Try creating all adapters to catch missing env vars, bad credentials, etc.
	sched := scheduler.New(cfg, log)
	failed := 0

	for _, backup := range cfg.Backups {
		log.Info("dry-run: checking adapters", "backup", backup.Name)
		if err := sched.ValidateAdapters(backup); err != nil {
			log.Error("adapter creation failed", "backup", backup.Name, "error", err)
			failed++
		} else {
			log.Info("dry-run: adapters OK", "backup", backup.Name)
		}
	}

	if failed > 0 {
		log.Error("dry-run failed", "failed", failed, "total", len(cfg.Backups))
		os.Exit(1)
	}

	log.Info("dry-run passed — all configs and adapters are valid")
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
