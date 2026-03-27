package retention

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Retention handles cleanup of old backup files
// Note: For S3, lifecycle policies should handle retention.
// This is mainly for local filesystem cleanup.
type Retention struct {
	basePath string
	days     int
}

// New creates a retention cleaner for the given path and age limit.
func New(basePath string, days int) *Retention {
	return &Retention{
		basePath: basePath,
		days:     days,
	}
}

// Cleanup removes files older than the configured retention period.
func (r *Retention) Cleanup() error {
	if r.days <= 0 {
		return nil // No retention policy
	}

	cutoffTime := time.Now().AddDate(0, 0, -r.days)

	return filepath.Walk(r.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove old file %s: %w", path, err)
			}
		}

		return nil
	})
}

