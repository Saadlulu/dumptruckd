package verify

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saadlulu/dumptruckd/internal/fileops"
)

// Downloader downloads a remote file to a local path.
type Downloader interface {
	Download(ctx context.Context, remotePath string, localPath string) error
}

// Verifier downloads and validates a backup file after upload.
type Verifier interface {
	Verify(ctx context.Context, remotePath string, dbType string, compressType string, encryptType string) error
}

// BackupVerifier implements Verifier by downloading, decrypting, decompressing,
// and running a format-specific integrity check on the backup file.
type BackupVerifier struct {
	downloader Downloader
	logger     *slog.Logger
}

// NewVerifier creates a new BackupVerifier.
func NewVerifier(downloader Downloader, logger *slog.Logger) *BackupVerifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &BackupVerifier{
		downloader: downloader,
		logger:     logger,
	}
}

// Verify downloads the uploaded backup, decompresses it, and validates its integrity.
// All temp files are cleaned up after verification completes or fails.
func (v *BackupVerifier) Verify(ctx context.Context, remotePath string, dbType string, compressType string, encryptType string) error {
	tmpDir, err := os.MkdirTemp("", "dumptruckd-verify-*")
	if err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Step 1: Download the uploaded file
	downloadPath := filepath.Join(tmpDir, "backup"+extensionForRemote(remotePath))
	if err := v.downloader.Download(ctx, remotePath, downloadPath); err != nil {
		return fmt.Errorf("download backup for verification: %w", err)
	}

	currentPath := downloadPath

	// Step 2: Decrypt if encrypted
	if encryptType == "age" || strings.HasSuffix(currentPath, ".age") {
		decrypted, err := fileops.DecryptAge(ctx, currentPath)
		if err != nil {
			return fmt.Errorf("decrypt backup (age): %w", err)
		}
		currentPath = decrypted
	} else if encryptType == "gpg" || strings.HasSuffix(currentPath, ".gpg") {
		decrypted, err := fileops.DecryptGpg(ctx, currentPath)
		if err != nil {
			return fmt.Errorf("decrypt backup (gpg): %w", err)
		}
		currentPath = decrypted
	}

	// Step 3: Decompress
	if compressType == "gzip" || strings.HasSuffix(currentPath, ".gz") {
		decompressed, err := fileops.DecompressGzip(ctx, currentPath)
		if err != nil {
			return fmt.Errorf("decompress backup: %w", err)
		}
		currentPath = decompressed
	}

	// Step 4: Validate based on database type
	if err := v.validate(ctx, currentPath, dbType); err != nil {
		return fmt.Errorf("backup validation failed: %w", err)
	}

	v.logger.Info("backup verification succeeded", "remote_path", remotePath, "db_type", dbType)
	return nil
}

// extensionForRemote extracts the file extension(s) from a remote path for naming the temp file.
func extensionForRemote(remotePath string) string {
	base := filepath.Base(remotePath)
	// Handle s3:// paths
	if strings.Contains(remotePath, "://") {
		parts := strings.SplitN(remotePath, "://", 2)
		if len(parts) == 2 {
			base = filepath.Base(parts[1])
		}
	}

	// Collect all extensions (e.g. .sql.gz.age)
	var exts []string
	for {
		ext := filepath.Ext(base)
		if ext == "" {
			break
		}
		exts = append([]string{ext}, exts...)
		base = strings.TrimSuffix(base, ext)
	}
	return strings.Join(exts, "")
}

// validate runs a format-specific integrity check on the backup file.
func (v *BackupVerifier) validate(ctx context.Context, filePath string, dbType string) error {
	switch dbType {
	case "postgres":
		return v.validatePostgres(ctx, filePath)
	default:
		// For unsupported types, do a basic file size check
		return v.validateFileSize(filePath)
	}
}

// validatePostgres checks that the file looks like a valid PostgreSQL dump.
// pg_dump in plain-text mode produces SQL files (not custom-format archives),
// so we check for the standard header comment rather than using pg_restore.
func (v *BackupVerifier) validatePostgres(ctx context.Context, filePath string) error {
	if err := v.validateFileSize(filePath); err != nil {
		return err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open dump file: %w", err)
	}
	defer f.Close()

	// Read the first 4KB — the header comment is at the top.
	buf := make([]byte, 4096)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Errorf("read dump file header: %w", err)
	}
	header := string(buf[:n])

	// Plain-text pg_dump output starts with "-- PostgreSQL database dump"
	if strings.Contains(header, "PostgreSQL database dump") {
		return nil
	}

	// Custom-format archives start with "PGDMP" magic bytes
	if strings.HasPrefix(header, "PGDMP") {
		return nil
	}

	return fmt.Errorf("file does not appear to be a valid PostgreSQL dump (missing header marker)")
}

// validateFileSize checks that the file is non-empty as a basic integrity check.
func (v *BackupVerifier) validateFileSize(filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat backup file: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("backup file is empty")
	}
	return nil
}
