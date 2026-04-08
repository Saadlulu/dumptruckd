package compress

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Saadlulu/dumptruckd/internal/fileops"
)

// GzipCompressor compresses files using gzip.
type GzipCompressor struct{}

// NewGzipCompressor creates a new gzip compressor.
func NewGzipCompressor() *GzipCompressor {
	return &GzipCompressor{}
}

func (c *GzipCompressor) Compress(ctx context.Context, inputPath string) (string, error) {
	outputPath := inputPath + ".gz"

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("open input file: %w", err)
	}
	defer inputFile.Close()

	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("create output file: %w", err)
	}

	gzipWriter := gzip.NewWriter(outputFile)

	if _, err := io.Copy(gzipWriter, fileops.ReaderWithContext(ctx, inputFile)); err != nil {
		gzipWriter.Close()
		outputFile.Close()
		os.Remove(outputPath)
		return "", fmt.Errorf("compress data: %w", err)
	}

	// Close gzip writer to flush the final block — errors here mean corrupt output
	if err := gzipWriter.Close(); err != nil {
		outputFile.Close()
		os.Remove(outputPath)
		return "", fmt.Errorf("finalize gzip: %w", err)
	}

	if err := outputFile.Close(); err != nil {
		os.Remove(outputPath)
		return "", fmt.Errorf("close output file: %w", err)
	}

	return outputPath, nil
}

// PassthroughCompressor returns files unchanged (no compression).
type PassthroughCompressor struct{}

// NewPassthroughCompressor creates a no-op compressor.
func NewPassthroughCompressor() *PassthroughCompressor {
	return &PassthroughCompressor{}
}

func (c *PassthroughCompressor) Compress(ctx context.Context, inputPath string) (string, error) {
	return inputPath, nil
}
