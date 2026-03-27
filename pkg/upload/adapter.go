package upload

import (
	"context"
	"fmt"

	"github.com/dumptruckd/dumptruckd/pkg/config"
)

// Uploader interface for upload adapters.
type Uploader interface {
	Upload(ctx context.Context, filePath string, backupName string) (string, error)
}

// VerifiableUploader extends Uploader with verify and delete capabilities for testing.
type VerifiableUploader interface {
	Uploader
	Verify(ctx context.Context, remotePath string) error
	Delete(ctx context.Context, remotePath string) error
}

// NewUploader creates an uploader based on config
func NewUploader(cfg config.UploadConfig) (Uploader, error) {
	switch cfg.Type {
	case "s3":
		return NewS3Uploader(cfg.S3)
	case "gcp":
		return nil, fmt.Errorf("gcp uploader not yet implemented")
	case "sftp":
		return nil, fmt.Errorf("sftp uploader not yet implemented")
	case "local":
		return NewLocalUploader(cfg.Path)
	default:
		return nil, fmt.Errorf("unknown upload type: %s", cfg.Type)
	}
}

