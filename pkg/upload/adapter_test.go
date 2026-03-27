package upload

import (
	"os"
	"strings"
	"testing"

	"github.com/dumptruckd/dumptruckd/pkg/config"
)

func TestNewUploader(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.UploadConfig
		envVars map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "local with path",
			cfg:     config.UploadConfig{Type: "local", Path: t.TempDir()},
			wantErr: false,
		},
		{
			name: "s3 valid",
			cfg: config.UploadConfig{
				Type: "s3",
				S3:   config.S3Config{Bucket: "test-bucket", Region: "us-east-1"},
			},
			envVars: map[string]string{
				"AWS_ACCESS_KEY_ID":     "test-key",
				"AWS_SECRET_ACCESS_KEY": "test-secret",
			},
			wantErr: false,
		},
		{
			name:    "s3 missing bucket",
			cfg:     config.UploadConfig{Type: "s3", S3: config.S3Config{}},
			wantErr: true,
			errMsg:  "bucket is required",
		},
		{
			name: "s3 missing env vars",
			cfg: config.UploadConfig{
				Type: "s3",
				S3:   config.S3Config{Bucket: "test-bucket"},
			},
			wantErr: true,
			errMsg:  "environment variables are required",
		},
		{
			name:    "gcp not implemented",
			cfg:     config.UploadConfig{Type: "gcp"},
			wantErr: true,
			errMsg:  "not yet implemented",
		},
		{
			name:    "sftp not implemented",
			cfg:     config.UploadConfig{Type: "sftp"},
			wantErr: true,
			errMsg:  "not yet implemented",
		},
		{
			name:    "unknown type",
			cfg:     config.UploadConfig{Type: "ftp"},
			wantErr: true,
			errMsg:  "unknown upload type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars for this test
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}
			// Clear AWS env vars for tests that shouldn't have them
			if tt.envVars == nil {
				os.Unsetenv("AWS_ACCESS_KEY_ID")
				os.Unsetenv("AWS_SECRET_ACCESS_KEY")
			}

			_, err := NewUploader(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewUploader() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("NewUploader() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}
