package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/robfig/cron/v3"
)

// safeNamePattern matches characters that are safe for backup names used in file paths.
var safeNamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// Config is the top-level configuration for dumptruckd.
type Config struct {
	Includes    []string                   `toml:"include,omitempty"`
	Databases   map[string]DatabaseConfig  `toml:"database,omitempty"`
	Compressors map[string]CompressConfig  `toml:"compressor,omitempty"`
	Uploaders   map[string]UploadConfig    `toml:"uploader,omitempty"`
	Retentions  map[string]RetentionConfig `toml:"retention,omitempty"`
	Notifiers   map[string]NotifyConfig    `toml:"notifier,omitempty"`
	Backups     []BackupConfig             `toml:"backup"`
	Logging     LoggingConfig              `toml:"logging"`
	Health      HealthConfig               `toml:"health"`
}

// BackupConfig defines a single backup job with its schedule and component references.
type BackupConfig struct {
	Name      string `toml:"name"`
	Schedule  string `toml:"schedule"` // cron expression
	
	// Component references (by name) or inline config
	Database  DatabaseConfig  `toml:"database"`
	DatabaseRef string        `toml:"database_ref,omitempty"` // Reference to named database
	
	Compress  CompressConfig  `toml:"compress,omitempty"`
	CompressRef string        `toml:"compress_ref,omitempty"` // Reference to named compressor
	
	Upload    UploadConfig    `toml:"upload"`
	UploadRef string          `toml:"upload_ref,omitempty"` // Reference to named uploader
	
	Retention RetentionConfig `toml:"retention,omitempty"`
	RetentionRef string      `toml:"retention_ref,omitempty"` // Reference to named retention
	
	Notify    NotifyConfig    `toml:"notify,omitempty"`
	NotifyRef string          `toml:"notify_ref,omitempty"` // Reference to named notifier
}

// DatabaseConfig defines a database connection for dumping.
type DatabaseConfig struct {
	Type     string `toml:"type"` // postgres, mysql, mongodb, sqlite, redis
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Database string `toml:"database"`
	Username string `toml:"username"`
}

// CompressConfig defines compression settings.
type CompressConfig struct {
	Type string `toml:"type"` // gzip, zstd, xz, none
}

// UploadConfig defines an upload destination.
type UploadConfig struct {
	Type     string `toml:"type"` // s3, gcp, sftp, local
	S3       S3Config `toml:"s3,omitempty"`
	Path     string `toml:"path,omitempty"` // for local or sftp
}

// S3Config defines S3-specific upload settings.
type S3Config struct {
	Bucket    string `toml:"bucket"`
	Region    string `toml:"region"`
	Prefix    string `toml:"prefix,omitempty"`
	Endpoint  string `toml:"endpoint,omitempty"`
}

// NotifyConfig defines notification settings for a backup job.
type NotifyConfig struct {
	Type   string      `toml:"type"` // slack, email, discord, webhook, none
	Slack  SlackConfig `toml:"slack,omitempty"`
	Webhook WebhookConfig `toml:"webhook,omitempty"`
}

// SlackConfig defines Slack webhook notification settings.
type SlackConfig struct {
	WebhookURL string `toml:"webhook_url"` // Falls back to SLACK_WEBHOOK_URL env var
}

// WebhookConfig defines generic webhook notification settings.
type WebhookConfig struct {
	URL           string `toml:"url"`
	// AllowInsecure permits plain HTTP webhooks. WARNING: notification payloads
	// include backup names, file paths, and error messages. Only use on trusted
	// networks where TLS termination happens upstream (e.g. behind a reverse proxy).
	AllowInsecure bool   `toml:"allow_insecure,omitempty"`
}

// RetentionConfig defines how long backups are kept.
type RetentionConfig struct {
	Days int `toml:"days"` // Keep last N days (S3 lifecycle handles this, but can be used for local)
}

// LoggingConfig defines structured logging settings.
type LoggingConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
	Output string `toml:"output"`
}

// HealthConfig configures the health check and metrics endpoints.
type HealthConfig struct {
	Enabled bool   `toml:"enabled"`
	Port    int    `toml:"port"`
	Token   string `toml:"token,omitempty"` // Bearer token for endpoint auth (or use HEALTH_BEARER_TOKEN env var)
}

// Load reads and parses a configuration file, resolving includes and references.
func Load(path string) (*Config, error) {
	// Warn if config file is world-readable (may contain sensitive paths/settings)
	if info, err := os.Stat(path); err == nil {
		if info.Mode().Perm()&0044 != 0 {
			fmt.Fprintf(os.Stderr, "WARNING: config file %s is readable by others (mode %o). Consider: chmod 640 %s\n", path, info.Mode().Perm(), path)
		}
	}
	return LoadWithBaseDir(path, filepath.Dir(path))
}

// LoadWithBaseDir reads config with a custom base directory for resolving relative paths.
func LoadWithBaseDir(path string, baseDir string) (*Config, error) {
	cfg := &Config{
		Databases:   make(map[string]DatabaseConfig),
		Compressors: make(map[string]CompressConfig),
		Uploaders:   make(map[string]UploadConfig),
		Retentions:  make(map[string]RetentionConfig),
		Notifiers:   make(map[string]NotifyConfig),
	}

	// Track loaded files to detect include cycles
	loaded := make(map[string]bool)

	// Load main config file
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	loaded[absPath] = true
	if err := loadConfigFile(path, cfg, baseDir); err != nil {
		return nil, err
	}

	// Load included files (with cycle detection)
	for _, includePath := range cfg.Includes {
		// Support relative paths from config directory
		if !filepath.IsAbs(includePath) {
			includePath = filepath.Join(baseDir, includePath)
		}

		// Support glob patterns
		if strings.Contains(includePath, "*") {
			matches, err := filepath.Glob(includePath)
			if err != nil {
				return nil, fmt.Errorf("glob pattern %s: %w", includePath, err)
			}
			for _, match := range matches {
				absMatch, _ := filepath.Abs(match)
				if loaded[absMatch] {
					continue // skip already-loaded files (cycle prevention)
				}
				loaded[absMatch] = true
				if err := loadConfigFile(match, cfg, baseDir); err != nil {
					return nil, fmt.Errorf("load included file %s: %w", match, err)
				}
			}
		} else {
			absInclude, _ := filepath.Abs(includePath)
			if loaded[absInclude] {
				continue // skip already-loaded files (cycle prevention)
			}
			loaded[absInclude] = true
			if err := loadConfigFile(includePath, cfg, baseDir); err != nil {
				return nil, fmt.Errorf("load included file %s: %w", includePath, err)
			}
		}
	}

	// Check for config.d/ directory pattern
	configDir := filepath.Join(baseDir, "config.d")
	if info, err := os.Stat(configDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(configDir)
		if err != nil {
			return nil, fmt.Errorf("read config.d directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
				continue
			}
			configPath := filepath.Join(configDir, entry.Name())
			absCP, _ := filepath.Abs(configPath)
			if loaded[absCP] {
				continue
			}
			loaded[absCP] = true
			if err := loadConfigFile(configPath, cfg, baseDir); err != nil {
				return nil, fmt.Errorf("load config.d file %s: %w", configPath, err)
			}
		}
	}

	// Resolve component references in backups
	if err := cfg.resolveReferences(); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "text"
	}
	if cfg.Logging.Output == "" {
		cfg.Logging.Output = "stdout"
	}

	return cfg, nil
}

func loadConfigFile(path string, cfg *Config, baseDir string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var fileCfg Config
	if err := toml.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	// Merge components (later files override earlier ones)
	if fileCfg.Databases != nil {
		if cfg.Databases == nil {
			cfg.Databases = make(map[string]DatabaseConfig)
		}
		for name, db := range fileCfg.Databases {
			cfg.Databases[name] = db
		}
	}
	if fileCfg.Compressors != nil {
		if cfg.Compressors == nil {
			cfg.Compressors = make(map[string]CompressConfig)
		}
		for name, comp := range fileCfg.Compressors {
			cfg.Compressors[name] = comp
		}
	}
	if fileCfg.Uploaders != nil {
		if cfg.Uploaders == nil {
			cfg.Uploaders = make(map[string]UploadConfig)
		}
		for name, up := range fileCfg.Uploaders {
			cfg.Uploaders[name] = up
		}
	}
	if fileCfg.Retentions != nil {
		if cfg.Retentions == nil {
			cfg.Retentions = make(map[string]RetentionConfig)
		}
		for name, ret := range fileCfg.Retentions {
			cfg.Retentions[name] = ret
		}
	}
	if fileCfg.Notifiers != nil {
		if cfg.Notifiers == nil {
			cfg.Notifiers = make(map[string]NotifyConfig)
		}
		for name, n := range fileCfg.Notifiers {
			cfg.Notifiers[name] = n
		}
	}

	// Merge backups (append)
	cfg.Backups = append(cfg.Backups, fileCfg.Backups...)

	// Merge logging (last file wins)
	if fileCfg.Logging.Level != "" {
		cfg.Logging = fileCfg.Logging
	}

	// Merge includes (append)
	cfg.Includes = append(cfg.Includes, fileCfg.Includes...)

	return nil
}

func (c *Config) resolveReferences() error {
	for i := range c.Backups {
		backup := &c.Backups[i]

		// Resolve database reference
		if backup.DatabaseRef != "" {
			db, ok := c.Databases[backup.DatabaseRef]
			if !ok {
				return fmt.Errorf("backup %s: database component '%s' not found", backup.Name, backup.DatabaseRef)
			}
			backup.Database = db
		}

		// Resolve compressor reference
		if backup.CompressRef != "" {
			comp, ok := c.Compressors[backup.CompressRef]
			if !ok {
				return fmt.Errorf("backup %s: compressor component '%s' not found", backup.Name, backup.CompressRef)
			}
			backup.Compress = comp
		}

		// Resolve uploader reference
		if backup.UploadRef != "" {
			up, ok := c.Uploaders[backup.UploadRef]
			if !ok {
				return fmt.Errorf("backup %s: uploader component '%s' not found", backup.Name, backup.UploadRef)
			}
			backup.Upload = up
		}

		// Resolve retention reference
		if backup.RetentionRef != "" {
			ret, ok := c.Retentions[backup.RetentionRef]
			if !ok {
				return fmt.Errorf("backup %s: retention component '%s' not found", backup.Name, backup.RetentionRef)
			}
			backup.Retention = ret
		}

		// Resolve notifier reference
		if backup.NotifyRef != "" {
			n, ok := c.Notifiers[backup.NotifyRef]
			if !ok {
				return fmt.Errorf("backup %s: notifier component '%s' not found", backup.Name, backup.NotifyRef)
			}
			backup.Notify = n
		}

		// Sanitize backup name to prevent directory traversal in upload paths
		if backup.Name != "" {
			backup.Name = safeNamePattern.ReplaceAllString(backup.Name, "_")
		}

		// Set defaults if not specified
		if backup.Compress.Type == "" {
			backup.Compress.Type = "gzip"
		}
		if backup.Upload.Type == "local" && backup.Upload.Path == "" {
			backup.Upload.Path = "/var/backups/dumptruckd"
		}

		// Normalize 5-field cron to 6-field by prepending seconds
		fields := strings.Fields(backup.Schedule)
		if len(fields) == 5 {
			backup.Schedule = "0 " + backup.Schedule
		}
	}

	return nil
}

// Validate checks that the configuration is complete and valid.
// It does not modify the config — defaults are applied in Load/resolveReferences.
func (c *Config) Validate() error {
	if len(c.Backups) == 0 {
		return fmt.Errorf("no backups configured")
	}

	knownDBTypes := map[string]bool{"postgres": true, "mysql": true, "mongodb": true, "sqlite": true, "redis": true}
	implementedDBTypes := map[string]bool{"postgres": true, "mysql": true}
	knownUploadTypes := map[string]bool{"s3": true, "gcp": true, "sftp": true, "local": true}
	implementedUploadTypes := map[string]bool{"s3": true, "local": true}
	knownCompressTypes := map[string]bool{"gzip": true, "zstd": true, "xz": true, "none": true, "": true}
	implementedCompressTypes := map[string]bool{"gzip": true, "none": true, "": true}
	knownNotifyTypes := map[string]bool{"slack": true, "webhook": true, "email": true, "discord": true, "none": true, "": true}
	cronParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	for i := range c.Backups {
		backup := &c.Backups[i]

		if backup.Name == "" {
			return fmt.Errorf("backup[%d]: name is required", i)
		}
		if backup.Schedule == "" {
			return fmt.Errorf("backup[%d] (%s): schedule is required", i, backup.Name)
		}

		// Normalize 5-field cron to 6-field (same as resolveReferences, needed when
		// Validate is called on configs not loaded through Load).
		fields := strings.Fields(backup.Schedule)
		if len(fields) == 5 {
			backup.Schedule = "0 " + backup.Schedule
		}

		// Validate cron expression (already normalized to 6-field in resolveReferences)
		if _, err := cronParser.Parse(backup.Schedule); err != nil {
			return fmt.Errorf("backup[%d] (%s): invalid schedule %q: %w (use cron format, e.g. \"0 2 * * *\" or \"0 0 2 * * *\")", i, backup.Name, backup.Schedule, err)
		}

		// Database validation
		if backup.Database.Type == "" {
			return fmt.Errorf("backup[%d] (%s): database.type is required", i, backup.Name)
		}
		if !knownDBTypes[backup.Database.Type] {
			return fmt.Errorf("backup[%d] (%s): unknown database type %q", i, backup.Name, backup.Database.Type)
		}
		if !implementedDBTypes[backup.Database.Type] {
			return fmt.Errorf("backup[%d] (%s): database type %q is not yet implemented", i, backup.Name, backup.Database.Type)
		}

		// Upload validation
		if backup.Upload.Type == "" {
			return fmt.Errorf("backup[%d] (%s): upload.type is required", i, backup.Name)
		}
		if !knownUploadTypes[backup.Upload.Type] {
			return fmt.Errorf("backup[%d] (%s): unknown upload type %q", i, backup.Name, backup.Upload.Type)
		}
		if !implementedUploadTypes[backup.Upload.Type] {
			return fmt.Errorf("backup[%d] (%s): upload type %q is not yet implemented", i, backup.Name, backup.Upload.Type)
		}
		if backup.Upload.Type == "s3" && backup.Upload.S3.Bucket == "" {
			return fmt.Errorf("backup[%d] (%s): s3.bucket is required when upload type is s3", i, backup.Name)
		}

		// Compress validation
		if !knownCompressTypes[backup.Compress.Type] {
			return fmt.Errorf("backup[%d] (%s): unknown compress type %q", i, backup.Name, backup.Compress.Type)
		}
		if !implementedCompressTypes[backup.Compress.Type] {
			return fmt.Errorf("backup[%d] (%s): compress type %q is not yet implemented", i, backup.Name, backup.Compress.Type)
		}

		// Port validation
		if backup.Database.Port != 0 && (backup.Database.Port < 1 || backup.Database.Port > 65535) {
			return fmt.Errorf("backup[%d] (%s): port must be between 1 and 65535", i, backup.Name)
		}

		// Notification validation
		if !knownNotifyTypes[backup.Notify.Type] {
			return fmt.Errorf("backup[%d] (%s): unknown notify type %q", i, backup.Name, backup.Notify.Type)
		}

		// Database-specific field validation
		switch backup.Database.Type {
		case "postgres", "mysql":
			if backup.Database.Host == "" {
				return fmt.Errorf("backup[%d] (%s): database.host is required for %s", i, backup.Name, backup.Database.Type)
			}
			if backup.Database.Database == "" {
				return fmt.Errorf("backup[%d] (%s): database.database is required for %s", i, backup.Name, backup.Database.Type)
			}
			if backup.Database.Username == "" {
				return fmt.Errorf("backup[%d] (%s): database.username is required for %s", i, backup.Name, backup.Database.Type)
			}
		}
	}

	return nil
}

