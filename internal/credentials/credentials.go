// Package credentials provides secure credential retrieval from environment variables.
package credentials

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// dangerousCredentialChars matches characters that could allow injection in
// credential files (newlines, backslashes).
var dangerousCredentialChars = regexp.MustCompile(`[\n\r\\]`)

// pgpassDangerousChars additionally rejects colons, which are the field
// delimiter in pgpass files.
var pgpassDangerousChars = regexp.MustCompile(`[\n\r\\:]`)

// envVarSanitizer strips characters that are not safe for environment variable names.
var envVarSanitizer = regexp.MustCompile(`[^A-Z0-9_]`)

// GetDBPassword retrieves the database password from environment variables.
// Checks DB_PASSWORD first, then DB_PASSWORD_{DBNAME} (uppercased, sanitized).
// The dbType parameter controls which characters are rejected: postgres rejects
// colons (pgpass delimiter) while other types only reject newlines and backslashes.
func GetDBPassword(dbName string, dbType string) (string, error) {
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
