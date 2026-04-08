package compress

import (
	"context"
	"fmt"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

// Compressor interface for compression adapters.
// Accepts a context so compression can be cancelled during graceful shutdown.
type Compressor interface {
	Compress(ctx context.Context, inputPath string) (string, error) // Returns path to compressed file
}

// NewCompressor creates a compressor based on config
func NewCompressor(cfg config.CompressConfig) (Compressor, error) {
	compressType := cfg.Type
	if compressType == "" {
		compressType = "gzip" // default
	}

	switch compressType {
	case "gzip":
		return NewGzipCompressor(), nil
	case "none":
		return NewPassthroughCompressor(), nil
	default:
		return nil, fmt.Errorf("unknown or unsupported compression type: %s", compressType)
	}
}
