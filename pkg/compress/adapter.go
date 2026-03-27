package compress

import (
	"fmt"

	"github.com/dumptruckd/dumptruckd/pkg/config"
)

// Compressor interface for compression adapters
type Compressor interface {
	Compress(inputPath string) (string, error) // Returns path to compressed file
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
	case "zstd":
		return nil, fmt.Errorf("zstd compressor not yet implemented")
	case "xz":
		return nil, fmt.Errorf("xz compressor not yet implemented")
	case "none":
		return NewPassthroughCompressor(), nil
	default:
		return nil, fmt.Errorf("unknown compression type: %s", compressType)
	}
}

