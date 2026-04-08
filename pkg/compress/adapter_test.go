package compress

import (
	"strings"
	"testing"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

func TestNewCompressor(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.CompressConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "gzip",
			cfg:     config.CompressConfig{Type: "gzip"},
			wantErr: false,
		},
		{
			name:    "none",
			cfg:     config.CompressConfig{Type: "none"},
			wantErr: false,
		},
		{
			name:    "empty type defaults to gzip",
			cfg:     config.CompressConfig{Type: ""},
			wantErr: false,
		},
		{
			name:    "unknown type",
			cfg:     config.CompressConfig{Type: "lz4"},
			wantErr: true,
			errMsg:  "unknown or unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCompressor(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCompressor() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("NewCompressor() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}
