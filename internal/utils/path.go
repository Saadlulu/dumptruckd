package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// SanitizePath ensures a path is safe and doesn't contain directory traversal
func SanitizePath(path string) string {
	// Remove any directory traversal attempts
	path = strings.ReplaceAll(path, "..", "")
	path = strings.ReplaceAll(path, "~", "")

	// Clean the path
	path = filepath.Clean(path)

	// Ensure it's absolute or relative to current directory
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(".", path)
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// BuildBackupPath constructs a backup file path: prefix/backupName/YYYY/MM/DD/filename
func BuildBackupPath(prefix string, backupName string, fileName string) string {
	datePath := FormatDatePath(Now())
	if prefix == "" {
		return filepath.Join(backupName, datePath, fileName)
	}
	return filepath.Join(prefix, backupName, datePath, fileName)
}

