package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SanitizePath ensures a path component is safe and doesn't contain directory traversal.
// It cleans the path first, then returns an error if the result contains ".." segments
// or starts with "~". This prevents bypass attacks like "....//".
func SanitizePath(path string) (string, error) {
	cleaned := filepath.Clean(path)

	parts := strings.Split(cleaned, string(filepath.Separator))
	for _, p := range parts {
		if p == ".." {
			return "", fmt.Errorf("path %q contains directory traversal", path)
		}
		if p == "~" || strings.HasPrefix(p, "~") {
			return "", fmt.Errorf("path %q contains home directory reference", path)
		}
	}

	return cleaned, nil
}

// EnsureDir creates a directory if it doesn't exist.
// Uses 0750 to prevent other users from reading backup directories.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0750)
}

// BuildBackupPath constructs a backup file path: prefix/backupName/YYYY/MM/DD/filename.
// All path components are sanitized to prevent directory traversal.
func BuildBackupPath(prefix string, backupName string, fileName string) (string, error) {
	backupName, err := SanitizePath(backupName)
	if err != nil {
		return "", fmt.Errorf("invalid backup name: %w", err)
	}
	fileName, err = SanitizePath(fileName)
	if err != nil {
		return "", fmt.Errorf("invalid file name: %w", err)
	}
	datePath := FormatDatePath(Now())
	if prefix == "" {
		return filepath.Join(backupName, datePath, fileName), nil
	}
	prefix, err = SanitizePath(prefix)
	if err != nil {
		return "", fmt.Errorf("invalid prefix: %w", err)
	}
	return filepath.Join(prefix, backupName, datePath, fileName), nil
}
