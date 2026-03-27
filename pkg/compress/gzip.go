package compress

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

// GzipCompressor compresses files using gzip.
type GzipCompressor struct{}

// NewGzipCompressor creates a new gzip compressor.
func NewGzipCompressor() *GzipCompressor {
	return &GzipCompressor{}
}

func (c *GzipCompressor) Compress(inputPath string) (string, error) {
	outputPath := inputPath + ".gz"

	// Open input file
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("open input file: %w", err)
	}
	defer inputFile.Close()

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("create output file: %w", err)
	}
	defer outputFile.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(outputFile)
	defer gzipWriter.Close()

	// Copy data
	if _, err := io.Copy(gzipWriter, inputFile); err != nil {
		os.Remove(outputPath)
		return "", fmt.Errorf("compress data: %w", err)
	}

	return outputPath, nil
}

// PassthroughCompressor returns files unchanged (no compression).
type PassthroughCompressor struct{}

// NewPassthroughCompressor creates a no-op compressor.
func NewPassthroughCompressor() *PassthroughCompressor {
	return &PassthroughCompressor{}
}

func (c *PassthroughCompressor) Compress(inputPath string) (string, error) {
	// Just return the input path unchanged
	return inputPath, nil
}

