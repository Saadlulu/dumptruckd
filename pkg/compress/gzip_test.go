package compress

import (
	"compress/gzip"
	"os"
	"strings"
	"testing"
)

func TestGzipCompressor_Compress_ValidFile(t *testing.T) {
	// Create a temp file with content
	tmpFile, err := os.CreateTemp("", "gzip-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "hello world, this is test data for gzip compression"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	tmpFile.Close()

	compressor := NewGzipCompressor()
	outputPath, err := compressor.Compress(tmpFile.Name())
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	defer os.Remove(outputPath)

	// Output should have .gz extension
	if !strings.HasSuffix(outputPath, ".gz") {
		t.Errorf("Compress() output path should end with .gz, got %q", outputPath)
	}

	// Output file should exist
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Output file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("Output file should not be empty")
	}
}

func TestGzipCompressor_Compress_OutputIsValidGzip(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "gzip-valid-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "test data to verify gzip validity"
	tmpFile.WriteString(content)
	tmpFile.Close()

	compressor := NewGzipCompressor()
	outputPath, err := compressor.Compress(tmpFile.Name())
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	defer os.Remove(outputPath)

	// Verify it's valid gzip by opening with gzip reader
	f, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("Failed to open compressed file: %v", err)
	}
	defer f.Close()

	reader, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("Not valid gzip: %v", err)
	}
	reader.Close()
}

func TestGzipCompressor_Compress_NonexistentFile(t *testing.T) {
	compressor := NewGzipCompressor()
	_, err := compressor.Compress("/nonexistent/file.txt")
	if err == nil {
		t.Error("Compress() should error for nonexistent input file")
	}
}

func TestPassthroughCompressor_ReturnsSamePath(t *testing.T) {
	compressor := NewPassthroughCompressor()
	input := "/some/path/dump.sql"
	output, err := compressor.Compress(input)
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if output != input {
		t.Errorf("Passthrough should return same path, got %q want %q", output, input)
	}
}
