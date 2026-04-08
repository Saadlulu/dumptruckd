package encrypt

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// AgeEncryptor encrypts files using the age encryption tool.
type AgeEncryptor struct {
	recipient string
}

// NewAgeEncryptor creates a new age encryptor.
// It validates that the age binary is available and the recipient env var is set.
func NewAgeEncryptor() (*AgeEncryptor, error) {
	if _, err := exec.LookPath("age"); err != nil {
		return nil, fmt.Errorf("age not found in PATH — install age to use encryption")
	}

	recipient := os.Getenv("DUMPTRUCKD_AGE_RECIPIENT")
	if recipient == "" {
		return nil, fmt.Errorf("DUMPTRUCKD_AGE_RECIPIENT is required when encrypt.type is age")
	}

	return &AgeEncryptor{recipient: recipient}, nil
}

// Encrypt encrypts the file at inputPath using age and returns the encrypted file path.
// The encrypted file has .age appended to the original filename.
// The unencrypted file is removed after successful encryption.
func (e *AgeEncryptor) Encrypt(ctx context.Context, inputPath string) (string, error) {
	outputPath := inputPath + ".age"

	cmd := exec.CommandContext(ctx, "age", "-r", e.recipient, "-o", outputPath, inputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Clean up partial output on failure
		os.Remove(outputPath)
		return "", fmt.Errorf("age encrypt failed: %w: %s", err, stderr.String())
	}

	// Remove the unencrypted file after successful encryption
	if err := os.Remove(inputPath); err != nil {
		return "", fmt.Errorf("remove unencrypted file: %w", err)
	}

	return outputPath, nil
}
