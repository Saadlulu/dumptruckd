package upload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dumptruckd/dumptruckd/internal/utils"
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

	// Build destination path using shared utility
	fileName := filepath.Base(filePath)
	relativePath := utils.BuildBackupPath("", backupName, fileName)
	destPath := filepath.Join(u.basePath, relativePath)
	destDir := filepath.Dir(destPath)

	if err := utils.EnsureDir(destDir); err != nil {
		return "", fmt.Errorf("create destination directory: %w", err)
	}

	destFile, err := os.Create(destPath)
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
func (u *LocalUploader) Verify(ctx context.Context, remotePath string) error {
	if _, err := os.Stat(remotePath); err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	return nil
}

// Delete removes a file from local filesystem.
func (u *LocalUploader) Delete(ctx context.Context, remotePath string) error {
	if err := os.Remove(remotePath); err != nil {
		return fmt.Errorf("delete file failed: %w", err)
	}
	return nil
}
