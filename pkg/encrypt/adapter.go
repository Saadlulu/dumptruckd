package encrypt

import (
	"context"
	"fmt"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

// Encryptor encrypts a file and returns the path to the encrypted file.
type Encryptor interface {
	Encrypt(ctx context.Context, inputPath string) (string, error)
}

// NewEncryptor creates an encryptor based on config.
func NewEncryptor(cfg config.EncryptConfig) (Encryptor, error) {
	switch cfg.Type {
	case "age":
		return NewAgeEncryptor()
	case "gpg":
		return NewGpgEncryptor()
	case "none", "":
		return NewPassthroughEncryptor(), nil
	default:
		return nil, fmt.Errorf("unknown encryption type: %s", cfg.Type)
	}
}

// PassthroughEncryptor returns files unchanged (no encryption).
type PassthroughEncryptor struct{}

// NewPassthroughEncryptor creates a no-op encryptor.
func NewPassthroughEncryptor() *PassthroughEncryptor {
	return &PassthroughEncryptor{}
}

// Encrypt returns the input path unchanged.
func (e *PassthroughEncryptor) Encrypt(ctx context.Context, inputPath string) (string, error) {
	return inputPath, nil
}
