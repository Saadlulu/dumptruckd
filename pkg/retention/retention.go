package retention

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Saadlulu/dumptruckd/internal/utils"
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
// Collects all errors and continues rather than stopping on the first failure.
// Note: uses file modification time (ModTime) for age comparison. If files are
// touched after creation (e.g. by verify operations), they may escape retention.
// For more precise control, consider embedding timestamps in filenames.
func (r *Retention) Cleanup() error {
	if r.days <= 0 {
		return nil // No retention policy
	}

	// Verify the base path exists before walking
	if _, err := os.Stat(r.basePath); err != nil {
		return fmt.Errorf("retention base path: %w", err)
	}

	cutoffTime := utils.Now().AddDate(0, 0, -r.days)
	var errs []error

	err := filepath.WalkDir(r.basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, fmt.Errorf("access %s: %w", path, err))
			return nil // continue walking
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			errs = append(errs, fmt.Errorf("stat %s: %w", path, err))
			return nil
		}

		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				errs = append(errs, fmt.Errorf("remove old file %s: %w", path, err))
			}
		}

		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}
