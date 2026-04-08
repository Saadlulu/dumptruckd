package restore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocalLister lists backup files from the local filesystem.
type LocalLister struct {
	basePath string
}

// NewLocalLister creates a new local filesystem lister.
func NewLocalLister(basePath string) *LocalLister {
	return &LocalLister{basePath: basePath}
}

// List returns all files under basePath whose path contains the given prefix.
// Files are returned as absolute paths.
func (l *LocalLister) List(ctx context.Context, prefix string) ([]string, error) {
	var files []string

	searchDir := filepath.Join(l.basePath, prefix)
	info, err := os.Stat(searchDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat backup directory: %w", err)
	}

	if !info.IsDir() {
		// prefix points to a single file
		return []string{searchDir}, nil
	}

	err = filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if d.IsDir() {
			return nil
		}
		// Only include files that look like backups (have common extensions)
		lower := strings.ToLower(d.Name())
		if strings.HasSuffix(lower, ".sql") ||
			strings.HasSuffix(lower, ".sql.gz") ||
			strings.HasSuffix(lower, ".sql.gz.age") ||
			strings.HasSuffix(lower, ".sql.gz.gpg") ||
			strings.HasSuffix(lower, ".gz") ||
			strings.HasSuffix(lower, ".age") ||
			strings.HasSuffix(lower, ".gpg") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk backup directory: %w", err)
	}

	return files, nil
}
