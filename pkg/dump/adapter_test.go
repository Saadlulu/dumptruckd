package dump

import (
	"testing"

	"github.com/Saadlulu/dumptruckd/pkg/config"
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

func TestGetDBPassword_FromDBPassword(t *testing.T) {
	t.Setenv("DB_PASSWORD", "secret123")

	password, err := getDBPassword("testdb", "postgres")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if password != "secret123" {
		t.Errorf("expected %q, got %q", "secret123", password)
	}
}

func TestGetDBPassword_FallbackToNamedVar(t *testing.T) {
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_PASSWORD_TESTDB", "named_secret")

	password, err := getDBPassword("testdb", "postgres")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if password != "named_secret" {
		t.Errorf("expected %q, got %q", "named_secret", password)
	}
}

func TestGetDBPassword_MissingBothVars(t *testing.T) {
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_PASSWORD_TESTDB", "")

	_, err := getDBPassword("testdb", "postgres")
	if err == nil {
		t.Fatal("expected error when both env vars are missing, got nil")
	}
	if !contains(err.Error(), "DB_PASSWORD") {
		t.Errorf("expected error to mention DB_PASSWORD, got %q", err.Error())
	}
}

func TestGetDBPassword_RejectsNewlineForPostgres(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass\nword")

	_, err := getDBPassword("testdb", "postgres")
	if err == nil {
		t.Fatal("expected error for password containing newline with postgres, got nil")
	}
	if !contains(err.Error(), "invalid characters") {
		t.Errorf("expected error about invalid characters, got %q", err.Error())
	}
}

func TestGetDBPassword_RejectsColonForPostgres(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass:word")

	_, err := getDBPassword("testdb", "postgres")
	if err == nil {
		t.Fatal("expected error for password containing colon with postgres, got nil")
	}
	if !contains(err.Error(), "invalid characters") {
		t.Errorf("expected error about invalid characters, got %q", err.Error())
	}
}

func TestGetDBPassword_AllowsColonForMySQL(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass:word")

	password, err := getDBPassword("testdb", "mysql")
	if err != nil {
		t.Fatalf("expected colon to be allowed for mysql, got error: %v", err)
	}
	if password != "pass:word" {
		t.Errorf("expected %q, got %q", "pass:word", password)
	}
}

func TestGetDBPassword_RejectsNewlineForMySQL(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass\nword")

	_, err := getDBPassword("testdb", "mysql")
	if err == nil {
		t.Fatal("expected error for password containing newline with mysql, got nil")
	}
	if !contains(err.Error(), "invalid characters") {
		t.Errorf("expected error about invalid characters, got %q", err.Error())
	}
}

func TestGetDBPassword_RejectsBackslashForBoth(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass\\word")

	_, err := getDBPassword("testdb", "postgres")
	if err == nil {
		t.Fatal("expected error for password containing backslash with postgres, got nil")
	}
	if !contains(err.Error(), "invalid characters") {
		t.Errorf("expected error about invalid characters for postgres, got %q", err.Error())
	}

	_, err = getDBPassword("testdb", "mysql")
	if err == nil {
		t.Fatal("expected error for password containing backslash with mysql, got nil")
	}
	if !contains(err.Error(), "invalid characters") {
		t.Errorf("expected error about invalid characters for mysql, got %q", err.Error())
	}
}

func TestGetDBPassword_SanitizesDBName(t *testing.T) {
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_PASSWORD_MY_DB", "sanitized_secret")

	password, err := getDBPassword("my-db", "postgres")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if password != "sanitized_secret" {
		t.Errorf("expected %q, got %q", "sanitized_secret", password)
	}
}
