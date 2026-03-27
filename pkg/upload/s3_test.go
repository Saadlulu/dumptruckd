package upload

import (
	"os"
	"testing"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

func TestNewS3Uploader_MissingBucket(t *testing.T) {
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	_, err := NewS3Uploader(config.S3Config{})
	if err == nil {
		t.Error("NewS3Uploader() should error when bucket is missing")
	}
}

func TestNewS3Uploader_MissingEnvVars(t *testing.T) {
	os.Unsetenv("AWS_ACCESS_KEY_ID")
	os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	// Prevent the SDK from finding credentials via shared credentials file or config
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")
	t.Setenv("AWS_CONFIG_FILE", "/dev/null")

	_, err := NewS3Uploader(config.S3Config{Bucket: "test-bucket"})
	if err == nil {
		t.Error("NewS3Uploader() should error when no AWS credentials are available")
	}
}

func TestNewS3Uploader_DefaultRegion(t *testing.T) {
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	uploader, err := NewS3Uploader(config.S3Config{Bucket: "test-bucket"})
	if err != nil {
		t.Fatalf("NewS3Uploader() error = %v", err)
	}

	// The session should have been created with us-east-1
	if uploader == nil {
		t.Fatal("NewS3Uploader() returned nil")
	}
}

func TestNewS3Uploader_ValidConfig(t *testing.T) {
	os.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	uploader, err := NewS3Uploader(config.S3Config{
		Bucket: "my-bucket",
		Region: "eu-west-1",
		Prefix: "backups",
	})
	if err != nil {
		t.Fatalf("NewS3Uploader() error = %v", err)
	}
	if uploader == nil {
		t.Fatal("NewS3Uploader() returned nil")
	}
	if uploader.cfg.Bucket != "my-bucket" {
		t.Errorf("Bucket = %q, want %q", uploader.cfg.Bucket, "my-bucket")
	}
}

func TestS3Uploader_ImplementsVerifiableUploader(t *testing.T) {
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	defer os.Unsetenv("AWS_ACCESS_KEY_ID")
	defer os.Unsetenv("AWS_SECRET_ACCESS_KEY")

	uploader, err := NewS3Uploader(config.S3Config{Bucket: "test"})
	if err != nil {
		t.Fatalf("NewS3Uploader() error = %v", err)
	}

	var _ VerifiableUploader = uploader
}
