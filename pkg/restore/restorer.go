package restore

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Saadlulu/dumptruckd/internal/credentials"
	"github.com/Saadlulu/dumptruckd/internal/fileops"
	"github.com/Saadlulu/dumptruckd/internal/utils"
	"github.com/Saadlulu/dumptruckd/pkg/config"
	"github.com/Saadlulu/dumptruckd/pkg/verify"
)

// RestoreOpts configures a restore operation.
type RestoreOpts struct {
	BackupName string
	Latest     bool
	Timestamp  string
}

// Lister lists objects at a given prefix in the upload destination.
type Lister interface {
	List(ctx context.Context, prefix string) ([]string, error)
}

// Restorer downloads a backup file and restores it into the database.
type Restorer struct {
	cfg        config.BackupConfig
	downloader verify.Downloader
	lister     Lister
	logger     *slog.Logger
}

// NewRestorer creates a new Restorer.
func NewRestorer(cfg config.BackupConfig, downloader verify.Downloader, lister Lister, logger *slog.Logger) *Restorer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Restorer{
		cfg:        cfg,
		downloader: downloader,
		lister:     lister,
		logger:     logger,
	}
}

// Restore downloads a backup and restores it into the database.
func (r *Restorer) Restore(ctx context.Context, opts RestoreOpts) error {
	if opts.BackupName == "" {
		return fmt.Errorf("backup name is required")
	}
	if !opts.Latest && opts.Timestamp == "" {
		return fmt.Errorf("either --latest or --timestamp must be specified")
	}

	// Sanitize the backup name to prevent directory traversal via CLI input.
	sanitized, err := utils.SanitizePath(opts.BackupName)
	if err != nil {
		return fmt.Errorf("invalid backup name: %w", err)
	}
	opts.BackupName = sanitized

	// Step 1: List objects matching the backup name prefix
	objects, err := r.lister.List(ctx, opts.BackupName)
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}
	if len(objects) == 0 {
		return fmt.Errorf("backup '%s' not found: no files at prefix", opts.BackupName)
	}

	// Step 2: Select the target file
	target, err := r.selectBackup(objects, opts)
	if err != nil {
		return err
	}

	r.logger.Info("selected backup for restore", "file", target, "mode", r.modeString(opts))

	// Step 3: Download, process, and restore with temp file cleanup
	tmpDir, err := os.MkdirTemp("", "dumptruckd-restore-*")
	if err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	downloadPath := filepath.Join(tmpDir, filepath.Base(target))
	if err := r.downloader.Download(ctx, target, downloadPath); err != nil {
		return fmt.Errorf("download backup: %w", err)
	}

	currentPath := downloadPath

	// Step 4: Decrypt if needed (check file extension)
	if strings.HasSuffix(currentPath, ".age") {
		decrypted, err := fileops.DecryptAge(ctx, currentPath)
		if err != nil {
			return fmt.Errorf("decrypt backup (age): %w", err)
		}
		currentPath = decrypted
	} else if strings.HasSuffix(currentPath, ".gpg") {
		decrypted, err := fileops.DecryptGpg(ctx, currentPath)
		if err != nil {
			return fmt.Errorf("decrypt backup (gpg): %w", err)
		}
		currentPath = decrypted
	}

	// Step 5: Decompress if needed
	if strings.HasSuffix(currentPath, ".gz") {
		decompressed, err := fileops.DecompressGzip(ctx, currentPath)
		if err != nil {
			return fmt.Errorf("decompress backup: %w", err)
		}
		currentPath = decompressed
	}

	// Step 6: Pipe into the appropriate restore tool
	if err := r.restoreDB(ctx, currentPath); err != nil {
		return fmt.Errorf("restore database: %w", err)
	}

	r.logger.Info("restore completed successfully", "backup", opts.BackupName, "file", target)
	return nil
}

// selectBackup picks the right backup file from the list based on opts.
func (r *Restorer) selectBackup(objects []string, opts RestoreOpts) (string, error) {
	if opts.Latest {
		// Sort lexicographically — backup filenames contain timestamps so
		// lexicographic order matches chronological order.
		sorted := make([]string, len(objects))
		copy(sorted, objects)
		sort.Strings(sorted)
		return sorted[len(sorted)-1], nil
	}

	// Match by timestamp substring
	for _, obj := range objects {
		if strings.Contains(filepath.Base(obj), opts.Timestamp) {
			return obj, nil
		}
	}

	return "", fmt.Errorf("backup '%s' not found at timestamp %s", opts.BackupName, opts.Timestamp)
}

func (r *Restorer) modeString(opts RestoreOpts) string {
	if opts.Latest {
		return "latest"
	}
	return "timestamp=" + opts.Timestamp
}

// restoreDB pipes the SQL file into psql or mysql depending on the database type.
func (r *Restorer) restoreDB(ctx context.Context, sqlPath string) error {
	password, err := credentials.GetDBPassword(r.cfg.Database.Database, r.cfg.Database.Type)
	if err != nil {
		return err
	}

	switch r.cfg.Database.Type {
	case "postgres":
		return r.restorePostgres(ctx, sqlPath, password)
	case "mysql":
		return r.restoreMySQL(ctx, sqlPath, password)
	default:
		return fmt.Errorf("unsupported database type for restore: %s", r.cfg.Database.Type)
	}
}

// restorePostgres pipes the SQL file into psql using a temp pgpass file
// to avoid exposing the password in /proc/<pid>/environ.
func (r *Restorer) restorePostgres(ctx context.Context, sqlPath string, password string) error {
	port := r.cfg.Database.Port
	if port == 0 {
		port = 5432
	}

	sqlFile, err := os.Open(sqlPath)
	if err != nil {
		return fmt.Errorf("open SQL file: %w", err)
	}
	defer sqlFile.Close()

	// Write password to a temp pgpass file to avoid exposing it in /proc/<pid>/environ.
	// psql reads credentials from PGPASSFILE.
	pgpassFile, err := os.CreateTemp("", "dumptruckd-pgpass-*")
	if err != nil {
		return fmt.Errorf("create pgpass file: %w", err)
	}
	defer os.Remove(pgpassFile.Name())
	if err := os.Chmod(pgpassFile.Name(), 0600); err != nil {
		pgpassFile.Close()
		return fmt.Errorf("restrict pgpass file permissions: %w", err)
	}
	// pgpass format: hostname:port:database:username:password
	fmt.Fprintf(pgpassFile, "%s:%d:%s:%s:%s\n",
		r.cfg.Database.Host, port, r.cfg.Database.Database, r.cfg.Database.Username, password)
	pgpassFile.Close()

	cmd := exec.CommandContext(ctx, "psql",
		"-h", r.cfg.Database.Host,
		"-p", fmt.Sprintf("%d", port),
		"-U", r.cfg.Database.Username,
		"-d", r.cfg.Database.Database,
	)
	cmd.Stdin = sqlFile
	// Minimal environment: only what psql needs.
	cmd.Env = []string{
		fmt.Sprintf("PGPASSFILE=%s", pgpassFile.Name()),
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("psql failed: %w\nstderr: %s", err, stderr.String())
	}

	return nil
}

// restoreMySQL pipes the SQL file into mysql using a defaults-extra-file
// to avoid exposing the password in /proc/<pid>/environ.
func (r *Restorer) restoreMySQL(ctx context.Context, sqlPath string, password string) error {
	port := r.cfg.Database.Port
	if port == 0 {
		port = 3306
	}

	sqlFile, err := os.Open(sqlPath)
	if err != nil {
		return fmt.Errorf("open SQL file: %w", err)
	}
	defer sqlFile.Close()

	// Write password to a temp defaults file to avoid exposing it in the process list.
	defaultsFile, err := os.CreateTemp("", "dumptruckd-my-*.cnf")
	if err != nil {
		return fmt.Errorf("create mysql defaults file: %w", err)
	}
	defer os.Remove(defaultsFile.Name())
	if err := os.Chmod(defaultsFile.Name(), 0600); err != nil {
		defaultsFile.Close()
		return fmt.Errorf("restrict mysql defaults file permissions: %w", err)
	}
	fmt.Fprintf(defaultsFile, "[client]\npassword=%s\n", password)
	defaultsFile.Close()

	cmd := exec.CommandContext(ctx, "mysql",
		fmt.Sprintf("--defaults-extra-file=%s", defaultsFile.Name()),
		fmt.Sprintf("-h%s", r.cfg.Database.Host),
		fmt.Sprintf("-P%d", port),
		fmt.Sprintf("-u%s", r.cfg.Database.Username),
		r.cfg.Database.Database,
	)
	cmd.Stdin = sqlFile
	// Minimal environment: only what mysql needs.
	cmd.Env = []string{
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysql failed: %w\nstderr: %s", err, stderr.String())
	}

	return nil
}
