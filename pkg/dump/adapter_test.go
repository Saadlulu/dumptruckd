package dump

import (
	"testing"

	"github.com/Saadlulu/dumptruckd/internal/credentials"
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
			cfg:     config.DatabaseConfig{Type: "postgres", Host: "localhost", Database: "testdb", Username: "user"},
			wantErr: false,
		},
		{
			name:    "unknown type",
			cfg:     config.DatabaseConfig{Type: "oracle"},
			wantErr: true,
			errMsg:  "unknown or unsupported",
		},
		{
			name:    "empty type",
			cfg:     config.DatabaseConfig{},
			wantErr: true,
			errMsg:  "unknown or unsupported",
		},
		{
			name:    "valid mysql",
			cfg:     config.DatabaseConfig{Type: "mysql", Host: "localhost", Database: "testdb", Username: "user"},
			wantErr: false,
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

	password, err := credentials.GetDBPassword("testdb", "postgres")
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

	password, err := credentials.GetDBPassword("testdb", "postgres")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if password != "named_secret" {
		t.Errorf("expected %q, got %q", "named_secret", password)
	}
}

func TestGetDBPassword_BothSet_UnsuffixedWins(t *testing.T) {
	t.Setenv("DB_PASSWORD", "global_secret")
	t.Setenv("DB_PASSWORD_TESTDB", "named_secret")

	password, err := credentials.GetDBPassword("testdb", "postgres")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if password != "global_secret" {
		t.Errorf("expected DB_PASSWORD to take precedence, got %q", password)
	}
}

func TestGetDBPassword_MissingBothVars(t *testing.T) {
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_PASSWORD_TESTDB", "")

	_, err := credentials.GetDBPassword("testdb", "postgres")
	if err == nil {
		t.Fatal("expected error when both env vars are missing, got nil")
	}
	if !contains(err.Error(), "DB_PASSWORD") {
		t.Errorf("expected error to mention DB_PASSWORD, got %q", err.Error())
	}
	if !contains(err.Error(), "DB_PASSWORD_TESTDB") {
		t.Errorf("expected error to mention DB_PASSWORD_TESTDB, got %q", err.Error())
	}
}

func TestGetDBPassword_MissingBothVars_SanitizedName(t *testing.T) {
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_PASSWORD_TRACEBIN_PRODUCTION", "")

	_, err := credentials.GetDBPassword("tracebin_production", "postgres")
	if err == nil {
		t.Fatal("expected error when both env vars are missing, got nil")
	}
	if !contains(err.Error(), "DB_PASSWORD_TRACEBIN_PRODUCTION") {
		t.Errorf("expected error to mention DB_PASSWORD_TRACEBIN_PRODUCTION, got %q", err.Error())
	}
}

func TestGetDBPassword_RejectsNewlineForPostgres(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass\nword")

	_, err := credentials.GetDBPassword("testdb", "postgres")
	if err == nil {
		t.Fatal("expected error for password containing newline with postgres, got nil")
	}
	if !contains(err.Error(), "invalid characters") {
		t.Errorf("expected error about invalid characters, got %q", err.Error())
	}
}

func TestGetDBPassword_RejectsColonForPostgres(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass:word")

	_, err := credentials.GetDBPassword("testdb", "postgres")
	if err == nil {
		t.Fatal("expected error for password containing colon with postgres, got nil")
	}
	if !contains(err.Error(), "invalid characters") {
		t.Errorf("expected error about invalid characters, got %q", err.Error())
	}
}

func TestGetDBPassword_AllowsColonForMySQL(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass:word")

	password, err := credentials.GetDBPassword("testdb", "mysql")
	if err != nil {
		t.Fatalf("expected colon to be allowed for mysql, got error: %v", err)
	}
	if password != "pass:word" {
		t.Errorf("expected %q, got %q", "pass:word", password)
	}
}

func TestGetDBPassword_RejectsNewlineForMySQL(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass\nword")

	_, err := credentials.GetDBPassword("testdb", "mysql")
	if err == nil {
		t.Fatal("expected error for password containing newline with mysql, got nil")
	}
	if !contains(err.Error(), "invalid characters") {
		t.Errorf("expected error about invalid characters, got %q", err.Error())
	}
}

func TestGetDBPassword_RejectsBackslashForBoth(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass\\word")

	_, err := credentials.GetDBPassword("testdb", "postgres")
	if err == nil {
		t.Fatal("expected error for password containing backslash with postgres, got nil")
	}
	if !contains(err.Error(), "invalid characters") {
		t.Errorf("expected error about invalid characters for postgres, got %q", err.Error())
	}

	_, err = credentials.GetDBPassword("testdb", "mysql")
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

	password, err := credentials.GetDBPassword("my-db", "postgres")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if password != "sanitized_secret" {
		t.Errorf("expected %q, got %q", "sanitized_secret", password)
	}
}
