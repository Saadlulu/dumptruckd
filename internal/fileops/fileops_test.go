package fileops

import (
	"compress/gzip"
	"context"
	"os"
	"strings"
	"testing"
)

func TestDecompressGzip_ValidFile(t *testing.T) {
	t.Parallel()

	// Create a valid gzip file
	tmpFile, err := os.CreateTemp("", "fileops-test-*.sql.gz")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	gzWriter := gzip.NewWriter(tmpFile)
	_, _ = gzWriter.Write([]byte("-- PostgreSQL database dump\nSELECT 1;\n"))
	gzWriter.Close()
	tmpFile.Close()

	result, err := DecompressGzip(context.Background(), tmpFile.Name())
	if err != nil {
		t.Fatalf("DecompressGzip() error = %v", err)
	}
	defer os.Remove(result)

	if !strings.HasSuffix(result, ".sql") {
		t.Errorf("DecompressGzip() result = %q, expected .sql suffix", result)
	}

	content, err := os.ReadFile(result)
	if err != nil {
		t.Fatalf("read decompressed file: %v", err)
	}
	if !strings.Contains(string(content), "PostgreSQL database dump") {
		t.Error("decompressed content should contain the original data")
	}
}

func TestDecompressGzip_NonexistentFile(t *testing.T) {
	t.Parallel()

	_, err := DecompressGzip(context.Background(), "/nonexistent/file.gz")
	if err == nil {
		t.Error("DecompressGzip() should error for nonexistent file")
	}
}

func TestDecompressGzip_InvalidGzip(t *testing.T) {
	t.Parallel()

	tmpFile, err := os.CreateTemp("", "fileops-bad-*.gz")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	_, _ = tmpFile.WriteString("this is not gzip data")
	tmpFile.Close()

	_, err = DecompressGzip(context.Background(), tmpFile.Name())
	if err == nil {
		t.Error("DecompressGzip() should error for invalid gzip data")
	}
}

func TestDecompressGzip_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Create a valid gzip file
	tmpFile, err := os.CreateTemp("", "fileops-ctx-*.sql.gz")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	gzWriter := gzip.NewWriter(tmpFile)
	_, _ = gzWriter.Write([]byte("test data"))
	gzWriter.Close()
	tmpFile.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = DecompressGzip(ctx, tmpFile.Name())
	if err == nil {
		t.Error("DecompressGzip() should error when context is cancelled")
	}
}

func TestReaderWithContext_PassesThrough(t *testing.T) {
	t.Parallel()

	r := strings.NewReader("hello world")
	cr := ReaderWithContext(context.Background(), r)

	buf := make([]byte, 11)
	n, err := cr.Read(buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if n != 11 || string(buf) != "hello world" {
		t.Errorf("Read() = %q, want %q", string(buf[:n]), "hello world")
	}
}

func TestReaderWithContext_CancelledContext(t *testing.T) {
	t.Parallel()

	r := strings.NewReader("hello world")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cr := ReaderWithContext(ctx, r)
	buf := make([]byte, 11)
	_, err := cr.Read(buf)
	if err == nil {
		t.Error("Read() should error when context is cancelled")
	}
}
