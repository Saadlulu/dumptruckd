package dump

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

// dangerousCredentialChars matches characters that could allow injection in
// credential files (newlines, backslashes).
var dangerousCredentialChars = regexp.MustCompile(`[\n\r\\]`)

// pgpassDangerousChars additionally rejects colons, which are the field
// delimiter in pgpass files.
var pgpassDangerousChars = regexp.MustCompile(`[\n\r\\:]`)

// envVarSanitizer strips characters that are not safe for environment variable names.
var envVarSanitizer = regexp.MustCompile(`[^A-Z0-9_]`)

// Dumper interface for database dump adapters
type Dumper interface {
	Dump(ctx context.Context) (string, error) // Returns path to dump file
}

// TestDumper extends Dumper with a lightweight test dump for config validation.
type TestDumper interface {
	Dumper
	TestDump(ctx context.Context) (string, error) // Returns path to small test dump file
}

// getDBPassword retrieves the database password from environment variables.
// Checks DB_PASSWORD first, then DB_PASSWORD_{DBNAME} (uppercased, sanitized).
// The dbType parameter controls which characters are rejected: postgres rejects
// colons (pgpass delimiter) while other types only reject newlines and backslashes.
func getDBPassword(dbName string, dbType string) (string, error) {
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		safeName := envVarSanitizer.ReplaceAllString(strings.ToUpper(dbName), "_")
		password = os.Getenv(fmt.Sprintf("DB_PASSWORD_%s", safeName))
	}
	if password == "" {
		safeName := envVarSanitizer.ReplaceAllString(strings.ToUpper(dbName), "_")
		return "", fmt.Errorf("DB_PASSWORD or DB_PASSWORD_%s environment variable not set", safeName)
	}
	// Postgres uses pgpass format where colons are field delimiters
	if dbType == "postgres" {
		if pgpassDangerousChars.MatchString(password) {
			return "", fmt.Errorf("DB_PASSWORD contains invalid characters for postgres (newlines, colons, or backslashes are not allowed in pgpass format)")
		}
	} else {
		if dangerousCredentialChars.MatchString(password) {
			return "", fmt.Errorf("DB_PASSWORD contains invalid characters (newlines or backslashes are not allowed)")
		}
	}
	return password, nil
}

// NewDumper creates a dumper based on database config
func NewDumper(cfg config.DatabaseConfig) (Dumper, error) {
	switch cfg.Type {
	case "postgres":
		return NewPostgresDumper(cfg)
	case "mysql":
		return NewMySQLDumper(cfg)
	case "mongodb":
		return nil, fmt.Errorf("mongodb dumper not yet implemented")
	case "sqlite":
		return nil, fmt.Errorf("sqlite dumper not yet implemented")
	case "redis":
		return nil, fmt.Errorf("redis dumper not yet implemented")
	default:
		return nil, fmt.Errorf("unknown database type: %s", cfg.Type)
	}
}

