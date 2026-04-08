// Package fileops provides shared file operations for decrypt and decompress
// used by both the restore and verify packages.
package fileops

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// DecryptAge decrypts an age-encrypted file. Returns the path to the decrypted file.
// Uses DUMPTRUCKD_AGE_IDENTITY env var for the identity file if set,
// otherwise falls back to the age default (~/.config/age/keys.txt).
func DecryptAge(ctx context.Context, inputPath string) (string, error) {
	outputPath := strings.TrimSuffix(inputPath, ".age")
	if outputPath == inputPath {
		outputPath = inputPath + ".decrypted"
	}

	args := []string{"--decrypt"}
	if identity := os.Getenv("DUMPTRUCKD_AGE_IDENTITY"); identity != "" {
		args = append(args, "-i", identity)
	}
	args = append(args, "-o", outputPath, inputPath)

	cmd := exec.CommandContext(ctx, "age", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("age decrypt failed: %w: %s", err, stderr.String())
	}

	return outputPath, nil
}

// DecryptGpg decrypts a GPG-encrypted file. Returns the path to the decrypted file.
func DecryptGpg(ctx context.Context, inputPath string) (string, error) {
	outputPath := strings.TrimSuffix(inputPath, ".gpg")
	if outputPath == inputPath {
		outputPath = inputPath + ".decrypted"
	}

	cmd := exec.CommandContext(ctx, "gpg", "--decrypt", "--output", outputPath, inputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gpg decrypt failed: %w: %s", err, stderr.String())
	}

	return outputPath, nil
}

// DecompressGzip decompresses a gzip file. Returns the path to the decompressed file.
func DecompressGzip(ctx context.Context, inputPath string) (string, error) {
	outputPath := strings.TrimSuffix(inputPath, ".gz")
	if outputPath == inputPath {
		outputPath = inputPath + ".decompressed"
	}

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("open compressed file: %w", err)
	}
	defer inputFile.Close()

	gzReader, err := gzip.NewReader(inputFile)
	if err != nil {
		return "", fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzReader.Close()

	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("create output file: %w", err)
	}

	if _, err := io.Copy(outputFile, readerWithContext(ctx, gzReader)); err != nil {
		outputFile.Close()
		os.Remove(outputPath)
		return "", fmt.Errorf("decompress data: %w", err)
	}

	if err := outputFile.Close(); err != nil {
		os.Remove(outputPath)
		return "", fmt.Errorf("close decompressed file: %w", err)
	}

	return outputPath, nil
}

// ReaderWithContext wraps a reader to check for context cancellation.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

// ReaderWithContext returns an io.Reader that checks for context cancellation
// before each read operation.
func ReaderWithContext(ctx context.Context, r io.Reader) io.Reader {
	return &ctxReader{ctx: ctx, r: r}
}

func readerWithContext(ctx context.Context, r io.Reader) io.Reader {
	return ReaderWithContext(ctx, r)
}

func (cr *ctxReader) Read(p []byte) (int, error) {
	select {
	case <-cr.ctx.Done():
		return 0, cr.ctx.Err()
	default:
		return cr.r.Read(p)
	}
}
