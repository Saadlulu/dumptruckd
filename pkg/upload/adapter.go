package upload

import (
	"context"
	"fmt"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

// Uploader interface for upload adapters.
type Uploader interface {
	Upload(ctx context.Context, filePath string, backupName string) (string, error)
	Verify(ctx context.Context, remotePath string) error
	Delete(ctx context.Context, remotePath string) error
}

// NewUploader creates an uploader based on config
func NewUploader(cfg config.UploadConfig) (Uploader, error) {
	switch cfg.Type {
	case "s3":
		return NewS3Uploader(cfg.S3)
	case "local":
		return NewLocalUploader(cfg.Path)
	default:
		return nil, fmt.Errorf("unknown or unsupported upload type: %s", cfg.Type)
	}
}
