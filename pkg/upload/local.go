package upload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saadlulu/dumptruckd/internal/utils"
)

// LocalUploader uploads backup files to the local filesystem.
type LocalUploader struct {
	basePath string
}

// NewLocalUploader creates a new local filesystem uploader.
func NewLocalUploader(basePath string) (*LocalUploader, error) {
	if basePath == "" {
		basePath = "/var/backups/dumptruckd"
	}

	return &LocalUploader{basePath: basePath}, nil
}

// Upload copies a file to the local backup directory structure.
func (u *LocalUploader) Upload(ctx context.Context, filePath string, backupName string) (string, error) {
	sourceFile, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open source file: %w", err)
	}
	defer sourceFile.Close()

	// Build destination path
	fileName := filepath.Base(filePath)
	relativePath, err := utils.BuildBackupPath("", backupName, fileName)
	if err != nil {
		return "", fmt.Errorf("build backup path: %w", err)
	}
	destPath := filepath.Join(u.basePath, relativePath)
	if err := u.validatePath(destPath); err != nil {
		return "", fmt.Errorf("destination path validation: %w", err)
	}
	destDir := filepath.Dir(destPath)

	if err := utils.EnsureDir(destDir); err != nil {
		return "", fmt.Errorf("create destination directory: %w", err)
	}

	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := destFile.ReadFrom(sourceFile); err != nil {
		os.Remove(destPath)
		return "", fmt.Errorf("copy file: %w", err)
	}

	return destPath, nil
}

// Verify checks if a file exists locally.
// The path must be under the configured base path.
func (u *LocalUploader) Verify(ctx context.Context, remotePath string) error {
	if err := u.validatePath(remotePath); err != nil {
		return err
	}
	if _, err := os.Stat(remotePath); err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	return nil
}

// Delete removes a file from local filesystem.
// The path must be under the configured base path.
func (u *LocalUploader) Delete(ctx context.Context, remotePath string) error {
	if err := u.validatePath(remotePath); err != nil {
		return err
	}
	if err := os.Remove(remotePath); err != nil {
		return fmt.Errorf("delete file failed: %w", err)
	}
	return nil
}

// validatePath ensures the given path is a descendant of the uploader's base path,
// preventing directory traversal attacks.
func (u *LocalUploader) validatePath(path string) error {
	absBase, err := filepath.Abs(u.basePath)
	if err != nil {
		return fmt.Errorf("resolve base path: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	// Ensure the path is under basePath (with trailing separator to avoid prefix tricks)
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return fmt.Errorf("path %q is outside base directory %q", path, u.basePath)
	}
	return nil
}
