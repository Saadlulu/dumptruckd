package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

// LoadFromEnv constructs a Config from DUMPTRUCKD_* environment variables.
// Returns nil, nil if DUMPTRUCKD_DB_TYPE is not set (env-var mode not active).
// Returns nil, error if DUMPTRUCKD_DB_TYPE is set but other required vars are missing.
func LoadFromEnv() (*Config, error) {
	dbType := os.Getenv("DUMPTRUCKD_DB_TYPE")
	if dbType == "" {
		return nil, nil
	}

	// Check required env vars
	required := map[string]string{
		"DUMPTRUCKD_DB_HOST":     os.Getenv("DUMPTRUCKD_DB_HOST"),
		"DUMPTRUCKD_DB_NAME":     os.Getenv("DUMPTRUCKD_DB_NAME"),
		"DUMPTRUCKD_DB_USER":     os.Getenv("DUMPTRUCKD_DB_USER"),
		"DUMPTRUCKD_UPLOAD_TYPE": os.Getenv("DUMPTRUCKD_UPLOAD_TYPE"),
	}

	var missing []string
	for name, val := range required {
		if val == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		// Sort for deterministic error messages
		slices.Sort(missing)
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	dbName := required["DUMPTRUCKD_DB_NAME"]

	// Build database config
	dbCfg := DatabaseConfig{
		Type:     dbType,
		Host:     required["DUMPTRUCKD_DB_HOST"],
		Database: dbName,
		Username: required["DUMPTRUCKD_DB_USER"],
	}

	// Parse optional port
	if portStr := os.Getenv("DUMPTRUCKD_DB_PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid DUMPTRUCKD_DB_PORT %q: %w", portStr, err)
		}
		dbCfg.Port = port
	}

	// Build upload config
	uploadCfg := UploadConfig{
		Type: required["DUMPTRUCKD_UPLOAD_TYPE"],
		S3: S3Config{
			Bucket:   os.Getenv("DUMPTRUCKD_S3_BUCKET"),
			Region:   envOrDefault("DUMPTRUCKD_S3_REGION", "us-east-1"),
			Prefix:   os.Getenv("DUMPTRUCKD_S3_PREFIX"),
			Endpoint: os.Getenv("DUMPTRUCKD_S3_ENDPOINT"),
		},
		Path: envOrDefault("DUMPTRUCKD_UPLOAD_PATH", "/var/backups/dumptruckd"),
	}

	// Build compress config
	compressCfg := CompressConfig{
		Type: envOrDefault("DUMPTRUCKD_COMPRESS_TYPE", "gzip"),
	}

	// Build notify config
	notifyCfg := NotifyConfig{
		Type: os.Getenv("DUMPTRUCKD_NOTIFY_TYPE"),
	}

	// Build retention config
	retentionCfg := RetentionConfig{}
	if daysStr := os.Getenv("DUMPTRUCKD_RETENTION_DAYS"); daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return nil, fmt.Errorf("invalid DUMPTRUCKD_RETENTION_DAYS %q: %w", daysStr, err)
		}
		retentionCfg.Days = days
	}
	if keepLastStr := os.Getenv("DUMPTRUCKD_RETENTION_KEEP_LAST"); keepLastStr != "" {
		keepLast, err := strconv.Atoi(keepLastStr)
		if err != nil {
			return nil, fmt.Errorf("invalid DUMPTRUCKD_RETENTION_KEEP_LAST %q: %w", keepLastStr, err)
		}
		retentionCfg.KeepLast = keepLast
	}

	// Build backup config
	backupName := os.Getenv("DUMPTRUCKD_BACKUP_NAME")
	if backupName == "" {
		backupName = dbName
	}

	schedule := envOrDefault("DUMPTRUCKD_SCHEDULE", "0 */6 * * *")

	// Build encrypt config
	encryptCfg := EncryptConfig{
		Type: os.Getenv("DUMPTRUCKD_ENCRYPT_TYPE"),
	}

	// Build hooks config
	hooksCfg := HooksConfig{
		Pre:  os.Getenv("DUMPTRUCKD_HOOK_PRE"),
		Post: os.Getenv("DUMPTRUCKD_HOOK_POST"),
	}

	// Parse size alert threshold
	var sizeAlertThreshold float64
	if threshStr := os.Getenv("DUMPTRUCKD_SIZE_ALERT_THRESHOLD"); threshStr != "" {
		thresh, err := strconv.ParseFloat(threshStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid DUMPTRUCKD_SIZE_ALERT_THRESHOLD %q: %w", threshStr, err)
		}
		sizeAlertThreshold = thresh
	}

	// Parse verify flag
	var verify bool
	if verifyStr := os.Getenv("DUMPTRUCKD_VERIFY"); verifyStr != "" {
		v, err := strconv.ParseBool(verifyStr)
		if err != nil {
			return nil, fmt.Errorf("invalid DUMPTRUCKD_VERIFY %q: %w", verifyStr, err)
		}
		verify = v
	}

	backup := BackupConfig{
		Name:               backupName,
		Schedule:           schedule,
		Database:           dbCfg,
		Compress:           compressCfg,
		Upload:             uploadCfg,
		Retention:          retentionCfg,
		Notify:             notifyCfg,
		Encrypt:            encryptCfg,
		Hooks:              hooksCfg,
		SizeAlertThreshold: sizeAlertThreshold,
		Verify:             verify,
	}

	// Build logging config
	loggingCfg := LoggingConfig{
		Level:  envOrDefault("DUMPTRUCKD_LOG_LEVEL", "info"),
		Format: envOrDefault("DUMPTRUCKD_LOG_FORMAT", "text"),
	}

	// Build health config
	healthCfg := HealthConfig{}
	if enabledStr := os.Getenv("DUMPTRUCKD_HEALTH_ENABLED"); enabledStr != "" {
		enabled, err := strconv.ParseBool(enabledStr)
		if err != nil {
			return nil, fmt.Errorf("invalid DUMPTRUCKD_HEALTH_ENABLED %q: %w", enabledStr, err)
		}
		healthCfg.Enabled = enabled
	}
	if portStr := os.Getenv("DUMPTRUCKD_HEALTH_PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid DUMPTRUCKD_HEALTH_PORT %q: %w", portStr, err)
		}
		healthCfg.Port = port
	} else if healthCfg.Enabled {
		healthCfg.Port = 8080
	}

	cfg := &Config{
		Backups: []BackupConfig{backup},
		Logging: loggingCfg,
		Health:  healthCfg,
	}

	// Validate using the same validation as TOML-loaded configs
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("env-var config validation failed: %w", err)
	}

	return cfg, nil
}

// envOrDefault returns the value of the environment variable named by key,
// or defaultVal if the variable is not set or empty.
func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
