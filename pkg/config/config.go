package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config is the top-level configuration for dumptruckd.
type Config struct {
	Includes    []string                  `toml:"include,omitempty"`
	Databases   map[string]DatabaseConfig `toml:"database,omitempty"`
	Compressors map[string]CompressConfig `toml:"compressor,omitempty"`
	Uploaders   map[string]UploadConfig   `toml:"uploader,omitempty"`
	Retentions  map[string]RetentionConfig `toml:"retention,omitempty"`
	Backups     []BackupConfig            `toml:"backup"`
	Logging     LoggingConfig             `toml:"logging"`
	Health      HealthConfig              `toml:"health"`
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
}

// DatabaseConfig defines a database connection for dumping.
type DatabaseConfig struct {
	Type     string `toml:"type"` // postgres, mysql, mongodb, sqlite, redis
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Database string `toml:"database"`
	Username string `toml:"username"`
	// Password should come from env var: DB_PASSWORD or DB_PASSWORD_{NAME}
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
	Endpoint  string `toml:"endpoint,omitempty"` // for S3-compatible services
	// Credentials from env: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
}

// NotifyConfig defines notification settings for a backup job.
type NotifyConfig struct {
	Type   string      `toml:"type"` // slack, email, discord, webhook, none
	Slack  SlackConfig `toml:"slack,omitempty"`
	Webhook WebhookConfig `toml:"webhook,omitempty"`
}

// SlackConfig defines Slack webhook notification settings.
type SlackConfig struct {
	WebhookURL string `toml:"webhook_url"`
	// Or use SLACK_WEBHOOK_URL env var
}

// WebhookConfig defines generic webhook notification settings.
type WebhookConfig struct {
	URL string `toml:"url"`
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
	Enabled bool `toml:"enabled"`
	Port    int  `toml:"port"`
}

// Load reads and parses a configuration file, resolving includes and references.
func Load(path string) (*Config, error) {
	return LoadWithBaseDir(path, filepath.Dir(path))
}

// LoadWithBaseDir reads config with a custom base directory for resolving relative paths.
func LoadWithBaseDir(path string, baseDir string) (*Config, error) {
	cfg := &Config{
		Databases:   make(map[string]DatabaseConfig),
		Compressors: make(map[string]CompressConfig),
		Uploaders:   make(map[string]UploadConfig),
		Retentions:  make(map[string]RetentionConfig),
	}

	// Load main config file
	if err := loadConfigFile(path, cfg, baseDir); err != nil {
		return nil, err
	}

	// Load included files
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
				if err := loadConfigFile(match, cfg, baseDir); err != nil {
					return nil, fmt.Errorf("load included file %s: %w", match, err)
				}
			}
		} else {
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

		// Set defaults if not specified
		if backup.Compress.Type == "" {
			backup.Compress.Type = "gzip"
		}
	}

	return nil
}

// Validate checks that the configuration is complete and valid.
func (c *Config) Validate() error {
	if len(c.Backups) == 0 {
		return fmt.Errorf("no backups configured")
	}

	knownDBTypes := map[string]bool{"postgres": true, "mysql": true, "mongodb": true, "sqlite": true, "redis": true}
	knownUploadTypes := map[string]bool{"s3": true, "gcp": true, "sftp": true, "local": true}
	knownCompressTypes := map[string]bool{"gzip": true, "zstd": true, "xz": true, "none": true, "": true}

	for i, backup := range c.Backups {
		if backup.Name == "" {
			return fmt.Errorf("backup[%d]: name is required", i)
		}
		if backup.Schedule == "" {
			return fmt.Errorf("backup[%d] (%s): schedule is required", i, backup.Name)
		}

		// Database validation
		if backup.Database.Type == "" {
			return fmt.Errorf("backup[%d] (%s): database.type is required", i, backup.Name)
		}
		if !knownDBTypes[backup.Database.Type] {
			return fmt.Errorf("backup[%d] (%s): unknown database type %q", i, backup.Name, backup.Database.Type)
		}

		// Upload validation
		if backup.Upload.Type == "" {
			return fmt.Errorf("backup[%d] (%s): upload.type is required", i, backup.Name)
		}
		if !knownUploadTypes[backup.Upload.Type] {
			return fmt.Errorf("backup[%d] (%s): unknown upload type %q", i, backup.Name, backup.Upload.Type)
		}
		if backup.Upload.Type == "s3" && backup.Upload.S3.Bucket == "" {
			return fmt.Errorf("backup[%d] (%s): s3.bucket is required when upload type is s3", i, backup.Name)
		}

		// Compress validation
		if !knownCompressTypes[backup.Compress.Type] {
			return fmt.Errorf("backup[%d] (%s): unknown compress type %q", i, backup.Name, backup.Compress.Type)
		}

		// Port validation
		if backup.Database.Port != 0 && (backup.Database.Port < 1 || backup.Database.Port > 65535) {
			return fmt.Errorf("backup[%d] (%s): port must be between 1 and 65535", i, backup.Name)
		}
	}

	return nil
}

