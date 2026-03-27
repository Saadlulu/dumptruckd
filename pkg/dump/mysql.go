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

// MySQLDumper creates database dumps using mysqldump.
type MySQLDumper struct {
	cfg config.DatabaseConfig
}

// NewMySQLDumper creates a new MySQL dumper with the given config.
func NewMySQLDumper(cfg config.DatabaseConfig) (*MySQLDumper, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("mysql host is required")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("mysql database name is required")
	}
	return &MySQLDumper{cfg: cfg}, nil
}

// Dump creates a full database dump.
func (d *MySQLDumper) Dump(ctx context.Context) (string, error) {
	return d.runMySQLDump(ctx, false)
}

// TestDump creates a small test dump (schema only, no data).
func (d *MySQLDumper) TestDump(ctx context.Context) (string, error) {
	return d.runMySQLDump(ctx, true)
}

func (d *MySQLDumper) getPassword() (string, error) {
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = os.Getenv(fmt.Sprintf("DB_PASSWORD_%s", d.cfg.Database))
	}
	if password == "" {
		return "", fmt.Errorf("DB_PASSWORD or DB_PASSWORD_%s environment variable not set", d.cfg.Database)
	}
	return password, nil
}

func (d *MySQLDumper) runMySQLDump(ctx context.Context, schemaOnly bool) (string, error) {
	password, err := d.getPassword()
	if err != nil {
		return "", err
	}

	tmpDir := os.TempDir()
	timestamp := utils.FormatTimestamp(utils.Now())
	prefix := "dump"
	if schemaOnly {
		prefix = "test_dump"
	}
	dumpFile := filepath.Join(tmpDir, fmt.Sprintf("%s_%s_%s.sql", prefix, d.cfg.Database, timestamp))

	port := d.cfg.Port
	if port == 0 {
		port = 3306
	}

	args := []string{
		fmt.Sprintf("-h%s", d.cfg.Host),
		fmt.Sprintf("-P%d", port),
		fmt.Sprintf("-u%s", d.cfg.Username),
		fmt.Sprintf("-p%s", password),
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

	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(dumpFile)
		return "", fmt.Errorf("mysqldump failed: %w\nOutput: %s", err, string(output))
	}

	return dumpFile, nil
}
