package dump

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/Saadlulu/dumptruckd/internal/credentials"
	"github.com/Saadlulu/dumptruckd/internal/utils"
	"github.com/Saadlulu/dumptruckd/pkg/config"
)

// MySQLDumper creates database dumps using mysqldump.
type MySQLDumper struct {
	cfg config.DatabaseConfig
	log *slog.Logger
}

// NewMySQLDumper creates a new MySQL dumper with the given config.
func NewMySQLDumper(cfg config.DatabaseConfig) (*MySQLDumper, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("mysql host is required")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("mysql database name is required")
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("mysql username is required")
	}
	return &MySQLDumper{cfg: cfg, log: slog.Default()}, nil
}

// Dump creates a full database dump.
func (d *MySQLDumper) Dump(ctx context.Context) (string, error) {
	return d.runMySQLDump(ctx, false)
}

// TestDump creates a small test dump (schema only, no data).
func (d *MySQLDumper) TestDump(ctx context.Context) (string, error) {
	return d.runMySQLDump(ctx, true)
}

func (d *MySQLDumper) runMySQLDump(ctx context.Context, schemaOnly bool) (string, error) {
	password, err := credentials.GetDBPassword(d.cfg.Database, "mysql")
	if err != nil {
		return "", err
	}

	prefix := "dump"
	if schemaOnly {
		prefix = "test_dump"
	}
	timestamp := utils.FormatTimestamp(utils.Now())
	pattern := fmt.Sprintf("%s_%s_%s_*.sql", prefix, d.cfg.Database, timestamp)

	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("create dump file: %w", err)
	}
	dumpFile := f.Name()
	if err := os.Chmod(dumpFile, 0600); err != nil {
		f.Close()
		os.Remove(dumpFile)
		return "", fmt.Errorf("restrict dump file permissions: %w", err)
	}
	f.Close()

	port := d.cfg.Port
	if port == 0 {
		port = 3306
	}

	// Write password to a temp defaults file to avoid exposing it in the process list.
	// mysqldump reads credentials from --defaults-extra-file.
	defaultsFile, err := os.CreateTemp("", "dumptruckd-my-*.cnf")
	if err != nil {
		return "", fmt.Errorf("create mysql defaults file: %w", err)
	}
	defer os.Remove(defaultsFile.Name())
	if err := os.Chmod(defaultsFile.Name(), 0600); err != nil {
		defaultsFile.Close()
		os.Remove(dumpFile)
		return "", fmt.Errorf("restrict mysql defaults file permissions: %w", err)
	}
	fmt.Fprintf(defaultsFile, "[client]\npassword=%s\n", password)
	defaultsFile.Close()

	args := []string{
		fmt.Sprintf("--defaults-extra-file=%s", defaultsFile.Name()),
		fmt.Sprintf("-h%s", d.cfg.Host),
		fmt.Sprintf("-P%d", port),
		fmt.Sprintf("-u%s", d.cfg.Username),
		"--single-transaction",
		"--routines",
		"--triggers",
		fmt.Sprintf("--result-file=%s", dumpFile),
	}
	if schemaOnly {
		args = append(args, "--no-data")
	}
	args = append(args, d.cfg.Database)

	cmd := exec.CommandContext(ctx, "mysqldump", args...)
	// Minimal environment: only what mysqldump needs.
	cmd.Env = []string{
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}

	var stopMonitor func()
	if !schemaOnly {
		d.log.Info(fmt.Sprintf("[dump] dumping %s (%s:%d)...", d.cfg.Database, d.cfg.Host, port))
		monitor := newProgressMonitor(dumpFile, d.log, d.cfg.Database, "mysqldump")
		stopMonitor = monitor.start(ctx)
	}

	output, err := cmd.CombinedOutput()

	if stopMonitor != nil {
		stopMonitor()
	}

	if err != nil {
		os.Remove(dumpFile)
		return "", fmt.Errorf("mysqldump failed: %w\nOutput: %s", err, string(output))
	}

	if info, statErr := os.Stat(dumpFile); statErr == nil && !schemaOnly {
		d.log.Info(fmt.Sprintf("[dump] completed: %s", formatBytes(info.Size())))
	}

	return dumpFile, nil
}
