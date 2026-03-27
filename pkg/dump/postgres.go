package dump

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dumptruckd/dumptruckd/internal/utils"
	"github.com/dumptruckd/dumptruckd/pkg/config"
)

// PostgresDumper creates database dumps using pg_dump.
type PostgresDumper struct {
	cfg config.DatabaseConfig
}

// NewPostgresDumper creates a new PostgreSQL dumper with the given config.
func NewPostgresDumper(cfg config.DatabaseConfig) (*PostgresDumper, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("postgres host is required")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("postgres database name is required")
	}
	return &PostgresDumper{cfg: cfg}, nil
}

// Dump creates a full database dump.
func (d *PostgresDumper) Dump(ctx context.Context) (string, error) {
	return d.runPgDump(ctx, false)
}

// TestDump creates a small test dump (schema only, no data).
func (d *PostgresDumper) TestDump(ctx context.Context) (string, error) {
	return d.runPgDump(ctx, true)
}

// getPassword retrieves the database password from environment variables.
func (d *PostgresDumper) getPassword() (string, error) {
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = os.Getenv(fmt.Sprintf("DB_PASSWORD_%s", d.cfg.Database))
	}
	if password == "" {
		return "", fmt.Errorf("DB_PASSWORD or DB_PASSWORD_%s environment variable not set", d.cfg.Database)
	}
	return password, nil
}

// runPgDump executes pg_dump with the configured options.
// If schemaOnly is true, only the schema is dumped (useful for testing).
func (d *PostgresDumper) runPgDump(ctx context.Context, schemaOnly bool) (string, error) {
	password, err := d.getPassword()
	if err != nil {
		return "", err
	}

	// Create temp file for dump
	tmpDir := os.TempDir()
	timestamp := utils.FormatTimestamp(utils.Now())
	prefix := "dump"
	if schemaOnly {
		prefix = "test_dump"
	}
	dumpFile := filepath.Join(tmpDir, fmt.Sprintf("%s_%s_%s.sql", prefix, d.cfg.Database, timestamp))

	// Build pg_dump command
	port := d.cfg.Port
	if port == 0 {
		port = 5432
	}

	args := []string{
		"-h", d.cfg.Host,
		"-p", fmt.Sprintf("%d", port),
		"-U", d.cfg.Username,
		"-d", d.cfg.Database,
		"-f", dumpFile,
		"--no-owner",
		"--no-acl",
	}
	if schemaOnly {
		args = append(args, "--schema-only")
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", password))

	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(dumpFile)
		return "", fmt.Errorf("pg_dump failed: %w\nOutput: %s", err, string(output))
	}

	return dumpFile, nil
}
