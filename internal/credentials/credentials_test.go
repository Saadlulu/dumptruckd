package credentials

import (
	"os"
	"strings"
	"testing"
)

func TestGetDBPassword_FromDBPassword(t *testing.T) {
	t.Setenv("DB_PASSWORD", "secret123")

	pw, err := GetDBPassword("mydb", "postgres")
	if err != nil {
		t.Fatalf("GetDBPassword() error = %v", err)
	}
	if pw != "secret123" {
		t.Errorf("GetDBPassword() = %q, want %q", pw, "secret123")
	}
}

func TestGetDBPassword_FromDBPasswordWithSuffix(t *testing.T) {
	os.Unsetenv("DB_PASSWORD")
	t.Setenv("DB_PASSWORD_MYDB", "suffixed_secret")

	pw, err := GetDBPassword("mydb", "postgres")
	if err != nil {
		t.Fatalf("GetDBPassword() error = %v", err)
	}
	if pw != "suffixed_secret" {
		t.Errorf("GetDBPassword() = %q, want %q", pw, "suffixed_secret")
	}
}

func TestGetDBPassword_PrefersDBPassword(t *testing.T) {
	t.Setenv("DB_PASSWORD", "global")
	t.Setenv("DB_PASSWORD_MYDB", "specific")

	pw, err := GetDBPassword("mydb", "postgres")
	if err != nil {
		t.Fatalf("GetDBPassword() error = %v", err)
	}
	if pw != "global" {
		t.Errorf("GetDBPassword() = %q, want %q (DB_PASSWORD should take precedence)", pw, "global")
	}
}

func TestGetDBPassword_Missing(t *testing.T) {
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_PASSWORD_TESTDB")

	_, err := GetDBPassword("testdb", "postgres")
	if err == nil {
		t.Error("GetDBPassword() should error when no password env var is set")
	}
	if !strings.Contains(err.Error(), "DB_PASSWORD") {
		t.Errorf("Error should mention DB_PASSWORD, got %q", err.Error())
	}
}

func TestGetDBPassword_PostgresRejectsColons(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass:word")

	_, err := GetDBPassword("mydb", "postgres")
	if err == nil {
		t.Error("GetDBPassword() should reject colons for postgres")
	}
}

func TestGetDBPassword_MySQLAllowsColons(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass:word")

	pw, err := GetDBPassword("mydb", "mysql")
	if err != nil {
		t.Fatalf("GetDBPassword() should allow colons for mysql, got error: %v", err)
	}
	if pw != "pass:word" {
		t.Errorf("GetDBPassword() = %q, want %q", pw, "pass:word")
	}
}

func TestGetDBPassword_RejectsNewlines(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass\nword")

	_, err := GetDBPassword("mydb", "mysql")
	if err == nil {
		t.Error("GetDBPassword() should reject newlines")
	}
}

func TestGetDBPassword_RejectsBackslashes(t *testing.T) {
	t.Setenv("DB_PASSWORD", "pass\\word")

	_, err := GetDBPassword("mydb", "mysql")
	if err == nil {
		t.Error("GetDBPassword() should reject backslashes")
	}
}

func TestGetDBPassword_SanitizesDBName(t *testing.T) {
	os.Unsetenv("DB_PASSWORD")
	t.Setenv("DB_PASSWORD_MY_DB", "sanitized_secret")

	pw, err := GetDBPassword("my-db", "postgres")
	if err != nil {
		t.Fatalf("GetDBPassword() error = %v", err)
	}
	if pw != "sanitized_secret" {
		t.Errorf("GetDBPassword() = %q, want %q", pw, "sanitized_secret")
	}
}
