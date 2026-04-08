package retention

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/Saadlulu/dumptruckd/internal/utils"
)

// Retention handles cleanup of old backup files
// Note: For S3, lifecycle policies should handle retention.
// This is mainly for local filesystem cleanup.
type Retention struct {
	basePath string
	days     int
	keepLast int
}

// New creates a retention cleaner for the given path, age limit, and count limit.
// A file is kept if it satisfies either condition: within days age OR within the
// top keepLast files by modification time (union policy).
// When keepLast is 0, only the days policy applies.
// When days is 0 and keepLast > 0, only the count policy applies.
// When both are 0 (or negative), no files are removed.
func New(basePath string, days int, keepLast int) *Retention {
	return &Retention{
		basePath: basePath,
		days:     days,
		keepLast: keepLast,
	}
}

// fileEntry holds a file's path and info for sorting.
type fileEntry struct {
	path string
	info fs.FileInfo
}

// Cleanup removes files that fall outside the configured retention policies.
// Collects all errors and continues rather than stopping on the first failure.
// Note: uses file modification time (ModTime) for age comparison. If files are
// touched after creation (e.g. by verify operations), they may escape retention.
// For more precise control, consider embedding timestamps in filenames.
func (r *Retention) Cleanup() error {
	if r.days <= 0 && r.keepLast <= 0 {
		return nil // No retention policy
	}

	// Verify the base path exists before walking
	if _, err := os.Stat(r.basePath); err != nil {
		return fmt.Errorf("retention base path: %w", err)
	}

	// Collect all files first so we can sort and apply count-based retention.
	var files []fileEntry
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

		files = append(files, fileEntry{path: path, info: info})
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}

	// Sort files by modification time descending (newest first).
	sort.Slice(files, func(i, j int) bool {
		return files[i].info.ModTime().After(files[j].info.ModTime())
	})

	// Build the set of files protected by count-based retention.
	keepByCount := make(map[string]bool)
	if r.keepLast > 0 {
		for i := 0; i < len(files) && i < r.keepLast; i++ {
			keepByCount[files[i].path] = true
		}
	}

	// Determine cutoff time for days-based retention.
	var cutoffTime = utils.Now().AddDate(0, 0, -r.days)

	for _, f := range files {
		keptByDays := r.days > 0 && !f.info.ModTime().Before(cutoffTime)
		keptByCount := keepByCount[f.path]

		// Union policy: keep if either condition is satisfied.
		if keptByDays || keptByCount {
			continue
		}

		if err := os.Remove(f.path); err != nil {
			errs = append(errs, fmt.Errorf("remove old file %s: %w", f.path, err))
		}
	}

	return errors.Join(errs...)
}
