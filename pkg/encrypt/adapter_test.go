package encrypt

import (
	"context"
	"os"
	"testing"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

func TestNewEncryptor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     config.EncryptConfig
		wantErr bool
	}{
		{"none returns passthrough", config.EncryptConfig{Type: "none"}, false},
		{"empty returns passthrough", config.EncryptConfig{Type: ""}, false},
		{"unknown type errors", config.EncryptConfig{Type: "aes256"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			enc, err := NewEncryptor(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewEncryptor() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && enc == nil {
				t.Error("NewEncryptor() returned nil for valid config")
			}
		})
	}
}

func TestPassthroughEncryptor_Encrypt(t *testing.T) {
	t.Parallel()

	enc := NewPassthroughEncryptor()
	out, err := enc.Encrypt(context.Background(), "/some/path.sql.gz")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if out != "/some/path.sql.gz" {
		t.Errorf("Encrypt() = %q, want %q", out, "/some/path.sql.gz")
	}
}

func TestNewAgeEncryptor_MissingRecipient(t *testing.T) {
	t.Parallel()

	os.Unsetenv("DUMPTRUCKD_AGE_RECIPIENT")
	_, err := NewAgeEncryptor()
	if err == nil {
		t.Error("NewAgeEncryptor() should error when DUMPTRUCKD_AGE_RECIPIENT is not set")
	}
}

func TestNewGpgEncryptor_MissingRecipient(t *testing.T) {
	t.Parallel()

	os.Unsetenv("DUMPTRUCKD_GPG_RECIPIENT")
	_, err := NewGpgEncryptor()
	if err == nil {
		t.Error("NewGpgEncryptor() should error when DUMPTRUCKD_GPG_RECIPIENT is not set")
	}
}
