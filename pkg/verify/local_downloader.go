package verify

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Saadlulu/dumptruckd/internal/fileops"
)

// LocalDownloader downloads files from the local filesystem by copying them.
type LocalDownloader struct{}

// NewLocalDownloader creates a new local filesystem downloader.
func NewLocalDownloader() *LocalDownloader {
	return &LocalDownloader{}
}

// Download copies a local file from remotePath to localPath.
func (d *LocalDownloader) Download(ctx context.Context, remotePath string, localPath string) error {
	src, err := os.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}

	if _, err := io.Copy(dst, fileops.ReaderWithContext(ctx, src)); err != nil {
		dst.Close()
		os.Remove(localPath)
		return fmt.Errorf("copy file: %w", err)
	}

	if err := dst.Close(); err != nil {
		os.Remove(localPath)
		return fmt.Errorf("close destination file: %w", err)
	}

	return nil
}
