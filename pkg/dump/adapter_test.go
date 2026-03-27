package dump

import (
	"testing"

	"github.com/dumptruckd/dumptruckd/pkg/config"
)

func TestNewDumper(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.DatabaseConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid postgres",
			cfg:     config.DatabaseConfig{Type: "postgres", Host: "localhost", Database: "testdb"},
			wantErr: false,
		},
		{
			name:    "unknown type",
			cfg:     config.DatabaseConfig{Type: "oracle"},
			wantErr: true,
			errMsg:  "unknown database type",
		},
		{
			name:    "empty type",
			cfg:     config.DatabaseConfig{},
			wantErr: true,
			errMsg:  "unknown database type",
		},
		{
			name:    "valid mysql",
			cfg:     config.DatabaseConfig{Type: "mysql", Host: "localhost", Database: "testdb"},
			wantErr: false,
		},
		{
			name:    "mongodb not implemented",
			cfg:     config.DatabaseConfig{Type: "mongodb"},
			wantErr: true,
			errMsg:  "not yet implemented",
		},
		{
			name:    "sqlite not implemented",
			cfg:     config.DatabaseConfig{Type: "sqlite"},
			wantErr: true,
			errMsg:  "not yet implemented",
		},
		{
			name:    "redis not implemented",
			cfg:     config.DatabaseConfig{Type: "redis"},
			wantErr: true,
			errMsg:  "not yet implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDumper(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDumper() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errMsg != "" && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("NewDumper() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
