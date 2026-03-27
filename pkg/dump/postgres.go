package dump

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/Saadlulu/dumptruckd/internal/utils"
	"github.com/Saadlulu/dumptruckd/pkg/config"
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

// runPgDump executes pg_dump with the configured options.
// If schemaOnly is true, only the schema is dumped (useful for testing).
func (d *PostgresDumper) runPgDump(ctx context.Context, schemaOnly bool) (string, error) {
	password, err := getDBPassword(d.cfg.Database, "postgres")
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
		port = 5432
	}

	// Write password to a temp pgpass file to avoid exposing it in /proc/<pid>/environ.
	// pg_dump reads credentials from PGPASSFILE.
	pgpassFile, err := os.CreateTemp("", "dumptruckd-pgpass-*")
	if err != nil {
		os.Remove(dumpFile)
		return "", fmt.Errorf("create pgpass file: %w", err)
	}
	defer os.Remove(pgpassFile.Name())
	if err := os.Chmod(pgpassFile.Name(), 0600); err != nil {
		pgpassFile.Close()
		os.Remove(dumpFile)
		return "", fmt.Errorf("restrict pgpass file permissions: %w", err)
	}
	// pgpass format: hostname:port:database:username:password
	fmt.Fprintf(pgpassFile, "%s:%d:%s:%s:%s\n", d.cfg.Host, port, d.cfg.Database, d.cfg.Username, password)
	pgpassFile.Close()

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
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSFILE=%s", pgpassFile.Name()))

	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(dumpFile)
		return "", fmt.Errorf("pg_dump failed: %w\nOutput: %s", err, string(output))
	}

	return dumpFile, nil
}
