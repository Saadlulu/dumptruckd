package encrypt

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// GpgEncryptor encrypts files using GPG.
type GpgEncryptor struct {
	recipient string
}

// NewGpgEncryptor creates a new GPG encryptor.
// It validates that the gpg binary is available and the recipient env var is set.
func NewGpgEncryptor() (*GpgEncryptor, error) {
	if _, err := exec.LookPath("gpg"); err != nil {
		return nil, fmt.Errorf("gpg not found in PATH — install gpg to use encryption")
	}

	recipient := os.Getenv("DUMPTRUCKD_GPG_RECIPIENT")
	if recipient == "" {
		return nil, fmt.Errorf("DUMPTRUCKD_GPG_RECIPIENT is required when encrypt.type is gpg")
	}

	return &GpgEncryptor{recipient: recipient}, nil
}

// Encrypt encrypts the file at inputPath using GPG and returns the encrypted file path.
// The encrypted file has .gpg appended to the original filename.
// The unencrypted file is removed after successful encryption.
func (e *GpgEncryptor) Encrypt(ctx context.Context, inputPath string) (string, error) {
	outputPath := inputPath + ".gpg"

	cmd := exec.CommandContext(ctx, "gpg", "--encrypt", "--recipient", e.recipient, "--output", outputPath, inputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Clean up partial output on failure
		os.Remove(outputPath)
		return "", fmt.Errorf("gpg encrypt failed: %w: %s", err, stderr.String())
	}

	// Remove the unencrypted file after successful encryption
	if err := os.Remove(inputPath); err != nil {
		return "", fmt.Errorf("remove unencrypted file: %w", err)
	}

	return outputPath, nil
}
