package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Load tests ---

func TestLoad_ValidSingleFile(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.toml")
	os.WriteFile(configFile, []byte(`
[[backup]]
name = "test-backup"
schedule = "0 0 * * * *"

[backup.database]
type = "postgres"
host = "localhost"
port = 5432
database = "mydb"
username = "user"

[backup.upload]
type = "local"
path = "/tmp/backups"
`), 0644)

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Backups) != 1 {
		t.Fatalf("Expected 1 backup, got %d", len(cfg.Backups))
	}
	if cfg.Backups[0].Name != "test-backup" {
		t.Errorf("Backup name = %q, want %q", cfg.Backups[0].Name, "test-backup")
	}
	if cfg.Backups[0].Database.Type != "postgres" {
		t.Errorf("Database type = %q, want %q", cfg.Backups[0].Database.Type, "postgres")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.toml")
	if err == nil {
		t.Error("Load() should error for missing file")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "bad.toml")
	os.WriteFile(configFile, []byte(`this is not valid toml [[[`), 0644)

	_, err := Load(configFile)
	if err == nil {
		t.Error("Load() should error for invalid TOML")
	}
}

func TestLoad_DefaultLoggingValues(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.toml")
	os.WriteFile(configFile, []byte(`
[[backup]]
name = "test"
schedule = "0 0 * * * *"
[backup.database]
type = "postgres"
host = "localhost"
database = "db"
[backup.upload]
type = "local"
path = "/tmp"
`), 0644)

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Default logging level = %q, want %q", cfg.Logging.Level, "info")
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Default logging format = %q, want %q", cfg.Logging.Format, "text")
	}
	if cfg.Logging.Output != "stdout" {
		t.Errorf("Default logging output = %q, want %q", cfg.Logging.Output, "stdout")
	}
}

func TestLoad_DefaultCompressType(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.toml")
	os.WriteFile(configFile, []byte(`
[[backup]]
name = "test"
schedule = "0 0 * * * *"
[backup.database]
type = "postgres"
host = "localhost"
database = "db"
[backup.upload]
type = "local"
path = "/tmp"
`), 0644)

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Backups[0].Compress.Type != "gzip" {
		t.Errorf("Default compress type = %q, want %q", cfg.Backups[0].Compress.Type, "gzip")
	}
}

// --- Modular config tests ---

func TestLoad_ModularConfigWithIncludes(t *testing.T) {
	dir := t.TempDir()

	// Create a sub-config file
	subDir := filepath.Join(dir, "conf.d")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "databases.toml"), []byte(`
[database.prod]
type = "postgres"
host = "prod-db.example.com"
port = 5432
database = "production"
username = "backup_user"
`), 0644)

	// Main config references the sub-config
	mainConfig := filepath.Join(dir, "main.toml")
	os.WriteFile(mainConfig, []byte(`
include = ["conf.d/databases.toml"]

[[backup]]
name = "prod-backup"
schedule = "0 0 2 * * *"
database_ref = "prod"
[backup.upload]
type = "local"
path = "/tmp/backups"
`), 0644)

	cfg, err := Load(mainConfig)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Database should be resolved from the included file
	if cfg.Backups[0].Database.Host != "prod-db.example.com" {
		t.Errorf("Database host = %q, want %q", cfg.Backups[0].Database.Host, "prod-db.example.com")
	}
}

func TestLoad_GlobPatternIncludes(t *testing.T) {
	dir := t.TempDir()

	subDir := filepath.Join(dir, "config.d")
	os.MkdirAll(subDir, 0755)

	os.WriteFile(filepath.Join(subDir, "db.toml"), []byte(`
[database.mydb]
type = "postgres"
host = "localhost"
database = "mydb"
`), 0644)

	os.WriteFile(filepath.Join(subDir, "upload.toml"), []byte(`
[uploader.local]
type = "local"
path = "/tmp/backups"
`), 0644)

	mainConfig := filepath.Join(dir, "main.toml")
	os.WriteFile(mainConfig, []byte(`
include = ["config.d/*.toml"]

[[backup]]
name = "test"
schedule = "0 0 * * * *"
database_ref = "mydb"
upload_ref = "local"
`), 0644)

	cfg, err := Load(mainConfig)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Databases) == 0 {
		t.Error("Databases should be loaded from glob includes")
	}
	if len(cfg.Uploaders) == 0 {
		t.Error("Uploaders should be loaded from glob includes")
	}
}

func TestLoad_ConfigDDirectory(t *testing.T) {
	dir := t.TempDir()

	// config.d/ is auto-discovered
	configD := filepath.Join(dir, "config.d")
	os.MkdirAll(configD, 0755)

	os.WriteFile(filepath.Join(configD, "compressors.toml"), []byte(`
[compressor.fast]
type = "gzip"
`), 0644)

	mainConfig := filepath.Join(dir, "main.toml")
	os.WriteFile(mainConfig, []byte(`
[[backup]]
name = "test"
schedule = "0 0 * * * *"
[backup.database]
type = "postgres"
host = "localhost"
database = "db"
[backup.upload]
type = "local"
path = "/tmp"
`), 0644)

	cfg, err := Load(mainConfig)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if _, ok := cfg.Compressors["fast"]; !ok {
		t.Error("Compressor from config.d/ should be loaded")
	}
}

func TestLoad_ConfigMerging_LaterOverrides(t *testing.T) {
	dir := t.TempDir()

	configD := filepath.Join(dir, "config.d")
	os.MkdirAll(configD, 0755)

	// First file defines a database
	os.WriteFile(filepath.Join(configD, "01-base.toml"), []byte(`
[database.mydb]
type = "postgres"
host = "old-host"
database = "mydb"
`), 0644)

	// Second file overrides it
	os.WriteFile(filepath.Join(configD, "02-override.toml"), []byte(`
[database.mydb]
type = "postgres"
host = "new-host"
database = "mydb"
`), 0644)

	mainConfig := filepath.Join(dir, "main.toml")
	os.WriteFile(mainConfig, []byte(`
[[backup]]
name = "test"
schedule = "0 0 * * * *"
database_ref = "mydb"
[backup.upload]
type = "local"
path = "/tmp"
`), 0644)

	cfg, err := Load(mainConfig)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Databases["mydb"].Host != "new-host" {
		t.Errorf("Database host = %q, want %q (later file should override)", cfg.Databases["mydb"].Host, "new-host")
	}
}

// --- Reference resolution tests ---

func TestResolveReferences_ValidDatabaseRef(t *testing.T) {
	cfg := &Config{
		Databases: map[string]DatabaseConfig{
			"prod": {Type: "postgres", Host: "prod-host", Database: "proddb"},
		},
		Uploaders:   make(map[string]UploadConfig),
		Compressors: make(map[string]CompressConfig),
		Retentions:  make(map[string]RetentionConfig),
		Backups: []BackupConfig{
			{Name: "test", DatabaseRef: "prod", Upload: UploadConfig{Type: "local"}},
		},
	}

	err := cfg.resolveReferences()
	if err != nil {
		t.Fatalf("resolveReferences() error = %v", err)
	}
	if cfg.Backups[0].Database.Host != "prod-host" {
		t.Errorf("Database host = %q, want %q", cfg.Backups[0].Database.Host, "prod-host")
	}
}

func TestResolveReferences_MissingDatabaseRef(t *testing.T) {
	cfg := &Config{
		Databases:   make(map[string]DatabaseConfig),
		Uploaders:   make(map[string]UploadConfig),
		Compressors: make(map[string]CompressConfig),
		Retentions:  make(map[string]RetentionConfig),
		Backups: []BackupConfig{
			{Name: "test", DatabaseRef: "nonexistent"},
		},
	}

	err := cfg.resolveReferences()
	if err == nil {
		t.Error("resolveReferences() should error for missing database ref")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("Error should mention the missing ref name, got %q", err.Error())
	}
}

func TestResolveReferences_MissingUploaderRef(t *testing.T) {
	cfg := &Config{
		Databases:   map[string]DatabaseConfig{"db": {Type: "postgres"}},
		Uploaders:   make(map[string]UploadConfig),
		Compressors: make(map[string]CompressConfig),
		Retentions:  make(map[string]RetentionConfig),
		Backups: []BackupConfig{
			{Name: "test", DatabaseRef: "db", UploadRef: "nonexistent"},
		},
	}

	err := cfg.resolveReferences()
	if err == nil {
		t.Error("resolveReferences() should error for missing uploader ref")
	}
}

func TestResolveReferences_MissingCompressorRef(t *testing.T) {
	cfg := &Config{
		Databases:   map[string]DatabaseConfig{"db": {Type: "postgres"}},
		Uploaders:   make(map[string]UploadConfig),
		Compressors: make(map[string]CompressConfig),
		Retentions:  make(map[string]RetentionConfig),
		Backups: []BackupConfig{
			{Name: "test", DatabaseRef: "db", CompressRef: "nonexistent"},
		},
	}

	err := cfg.resolveReferences()
	if err == nil {
		t.Error("resolveReferences() should error for missing compressor ref")
	}
}

func TestResolveReferences_MissingRetentionRef(t *testing.T) {
	cfg := &Config{
		Databases:   map[string]DatabaseConfig{"db": {Type: "postgres"}},
		Uploaders:   make(map[string]UploadConfig),
		Compressors: make(map[string]CompressConfig),
		Retentions:  make(map[string]RetentionConfig),
		Backups: []BackupConfig{
			{Name: "test", DatabaseRef: "db", RetentionRef: "nonexistent"},
		},
	}

	err := cfg.resolveReferences()
	if err == nil {
		t.Error("resolveReferences() should error for missing retention ref")
	}
}

// --- Validate tests ---

func TestValidate_NoBackups(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when no backups configured")
	}
}

func TestValidate_MissingName(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{Schedule: "0 0 * * * *", Database: DatabaseConfig{Type: "postgres"}, Upload: UploadConfig{Type: "local"}},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when backup name is missing")
	}
}

func TestValidate_MissingSchedule(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{Name: "test", Database: DatabaseConfig{Type: "postgres"}, Upload: UploadConfig{Type: "local"}},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when schedule is missing")
	}
}

func TestValidate_MissingDatabase(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{Name: "test", Schedule: "0 0 * * * *", Upload: UploadConfig{Type: "local"}},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when database is missing")
	}
}

func TestValidate_MissingUpload(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{Name: "test", Schedule: "0 0 * * * *", Database: DatabaseConfig{Type: "postgres"}},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when upload is missing")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres", Host: "localhost", Database: "db", Username: "user"},
				Upload:   UploadConfig{Type: "local"},
			},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() should pass for valid config, got %v", err)
	}
}

func TestValidate_UnknownDatabaseType(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "oracle"},
				Upload:   UploadConfig{Type: "local"},
				Compress: CompressConfig{Type: "gzip"},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error for unknown database type")
	}
	if !strings.Contains(err.Error(), "unknown database type") {
		t.Errorf("Error should mention unknown database type, got %q", err.Error())
	}
}

func TestValidate_UnknownUploadType(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres"},
				Upload:   UploadConfig{Type: "ftp"},
				Compress: CompressConfig{Type: "gzip"},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error for unknown upload type")
	}
}

func TestValidate_UnknownCompressType(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres"},
				Upload:   UploadConfig{Type: "local"},
				Compress: CompressConfig{Type: "lz4"},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error for unknown compress type")
	}
}

func TestValidate_S3MissingBucket(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres"},
				Upload:   UploadConfig{Type: "s3", S3: S3Config{}},
				Compress: CompressConfig{Type: "gzip"},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when s3 bucket is missing")
	}
	if !strings.Contains(err.Error(), "bucket") {
		t.Errorf("Error should mention bucket, got %q", err.Error())
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres", Port: 99999},
				Upload:   UploadConfig{Type: "local"},
				Compress: CompressConfig{Type: "gzip"},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error for invalid port")
	}
}

func TestValidate_ValidPortZero(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres", Host: "localhost", Database: "db", Username: "user", Port: 0},
				Upload:   UploadConfig{Type: "local"},
				Compress: CompressConfig{Type: "gzip"},
			},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() should pass for port 0 (default), got %v", err)
	}
}

func TestValidate_S3WithBucket(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres", Host: "localhost", Database: "db", Username: "user"},
				Upload:   UploadConfig{Type: "s3", S3: S3Config{Bucket: "my-bucket"}},
				Compress: CompressConfig{Type: "gzip"},
			},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() should pass for s3 with bucket, got %v", err)
	}
}

func TestValidate_InvalidCronExpression(t *testing.T) {
	tests := []struct {
		name     string
		schedule string
		wantErr  bool
	}{
		{"valid 6-field cron", "0 0 2 * * *", false},
		{"valid every minute", "0 * * * * *", false},
		{"valid every 6 hours", "0 0 */6 * * *", false},
		{"garbage", "not a cron", true},
		{"empty", "", true},
		{"5-field standard cron (auto-converted)", "0 2 * * *", false},
		{"too many fields", "0 0 2 * * * *", true},
		{"stars only no spaces", "******", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Backups: []BackupConfig{
					{
						Name:     "test",
						Schedule: tt.schedule,
						Database: DatabaseConfig{Type: "postgres", Host: "localhost", Database: "db", Username: "user"},
						Upload:   UploadConfig{Type: "local"},
						Compress: CompressConfig{Type: "gzip"},
					},
				},
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() with schedule %q: error = %v, wantErr %v", tt.schedule, err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.schedule != "" {
				if !strings.Contains(err.Error(), "schedule") {
					t.Errorf("Error should mention 'schedule', got %q", err.Error())
				}
			}
		})
	}
}

func TestValidate_PostgresMissingHost(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres", Database: "db", Username: "user"},
				Upload:   UploadConfig{Type: "local"},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when postgres host is missing")
	}
	if !strings.Contains(err.Error(), "host is required") {
		t.Errorf("Error should mention host, got %q", err.Error())
	}
}

func TestValidate_PostgresMissingDatabase(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres", Host: "localhost", Username: "user"},
				Upload:   UploadConfig{Type: "local"},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when postgres database is missing")
	}
	if !strings.Contains(err.Error(), "database is required") {
		t.Errorf("Error should mention database, got %q", err.Error())
	}
}

func TestValidate_PostgresMissingUsername(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres", Host: "localhost", Database: "db"},
				Upload:   UploadConfig{Type: "local"},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when postgres username is missing")
	}
	if !strings.Contains(err.Error(), "username is required") {
		t.Errorf("Error should mention username, got %q", err.Error())
	}
}

func TestValidate_MySQLMissingHost(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "mysql", Database: "db", Username: "user"},
				Upload:   UploadConfig{Type: "local"},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when mysql host is missing")
	}
}

func TestValidate_LocalUploadDefaultPath(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres", Host: "localhost", Database: "db", Username: "user"},
				Upload:   UploadConfig{Type: "local"},
			},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() should pass for local upload without path, got %v", err)
	}
}

// --- Notify reference resolution tests ---

func TestResolveReferences_ValidNotifyRef(t *testing.T) {
	cfg := &Config{
		Databases:   map[string]DatabaseConfig{"db": {Type: "postgres", Host: "localhost", Database: "mydb"}},
		Compressors: make(map[string]CompressConfig),
		Uploaders:   make(map[string]UploadConfig),
		Retentions:  make(map[string]RetentionConfig),
		Notifiers: map[string]NotifyConfig{
			"slack-team": {Type: "slack", Slack: SlackConfig{WebhookURL: "https://hooks.slack.com/test"}},
		},
		Backups: []BackupConfig{
			{
				Name:      "test",
				NotifyRef: "slack-team",
				Database:  DatabaseConfig{Type: "postgres", Host: "localhost", Database: "mydb"},
				Upload:    UploadConfig{Type: "local", Path: "/tmp"},
			},
		},
	}

	err := cfg.resolveReferences()
	if err != nil {
		t.Fatalf("resolveReferences() error = %v", err)
	}
	if cfg.Backups[0].Notify.Type != "slack" {
		t.Errorf("Notify.Type = %q, want %q", cfg.Backups[0].Notify.Type, "slack")
	}
	if cfg.Backups[0].Notify.Slack.WebhookURL != "https://hooks.slack.com/test" {
		t.Errorf("Notify.Slack.WebhookURL = %q, want %q", cfg.Backups[0].Notify.Slack.WebhookURL, "https://hooks.slack.com/test")
	}
}

func TestResolveReferences_MissingNotifyRef(t *testing.T) {
	cfg := &Config{
		Databases:   map[string]DatabaseConfig{"db": {Type: "postgres"}},
		Compressors: make(map[string]CompressConfig),
		Uploaders:   make(map[string]UploadConfig),
		Retentions:  make(map[string]RetentionConfig),
		Notifiers:   make(map[string]NotifyConfig),
		Backups: []BackupConfig{
			{Name: "test", NotifyRef: "nonexistent"},
		},
	}

	err := cfg.resolveReferences()
	if err == nil {
		t.Error("resolveReferences() should error for missing notify ref")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("Error should mention the missing ref name, got %q", err.Error())
	}
}

// --- Notify type validation tests ---

func TestValidate_UnknownNotifyType(t *testing.T) {
	cfg := &Config{
		Backups: []BackupConfig{
			{
				Name:     "test",
				Schedule: "0 0 * * * *",
				Database: DatabaseConfig{Type: "postgres", Host: "localhost", Database: "db", Username: "user"},
				Upload:   UploadConfig{Type: "local"},
				Notify:   NotifyConfig{Type: "teams"},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error for unknown notify type")
	}
	if !strings.Contains(err.Error(), "unknown notify type") {
		t.Errorf("Error should mention 'unknown notify type', got %q", err.Error())
	}
}

func TestValidate_ValidNotifyTypes(t *testing.T) {
	tests := []struct {
		name       string
		notifyType string
	}{
		{"slack", "slack"},
		{"webhook", "webhook"},
		{"none", "none"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Backups: []BackupConfig{
					{
						Name:     "test",
						Schedule: "0 0 * * * *",
						Database: DatabaseConfig{Type: "postgres", Host: "localhost", Database: "db", Username: "user"},
						Upload:   UploadConfig{Type: "local"},
						Notify:   NotifyConfig{Type: tt.notifyType},
					},
				},
			}
			err := cfg.Validate()
			if err != nil {
				t.Errorf("Validate() with notify type %q should pass, got %v", tt.notifyType, err)
			}
		})
	}
}

// --- Include cycle detection test ---

func TestLoad_IncludeCycleDetection(t *testing.T) {
	dir := t.TempDir()

	fileA := filepath.Join(dir, "a.toml")
	fileB := filepath.Join(dir, "b.toml")

	// a.toml includes b.toml
	os.WriteFile(fileA, []byte(`
include = ["b.toml"]

[[backup]]
name = "test"
schedule = "0 0 * * * *"
[backup.database]
type = "postgres"
host = "localhost"
database = "db"
username = "user"
[backup.upload]
type = "local"
path = "/tmp"
`), 0644)

	// b.toml includes a.toml (cycle)
	os.WriteFile(fileB, []byte(`
include = ["a.toml"]
`), 0644)

	cfg, err := Load(fileA)
	if err != nil {
		t.Fatalf("Load() should handle include cycles gracefully, got error: %v", err)
	}
	if len(cfg.Backups) != 1 {
		t.Errorf("Expected 1 backup, got %d", len(cfg.Backups))
	}
}

// --- Backup name sanitization test ---

func TestResolveReferences_BackupNameSanitized(t *testing.T) {
	cfg := &Config{
		Databases:   make(map[string]DatabaseConfig),
		Compressors: make(map[string]CompressConfig),
		Uploaders:   make(map[string]UploadConfig),
		Retentions:  make(map[string]RetentionConfig),
		Notifiers:   make(map[string]NotifyConfig),
		Backups: []BackupConfig{
			{
				Name:     "../../evil",
				Database: DatabaseConfig{Type: "postgres", Host: "localhost", Database: "db"},
				Upload:   UploadConfig{Type: "local", Path: "/tmp"},
			},
		},
	}

	err := cfg.resolveReferences()
	if err != nil {
		t.Fatalf("resolveReferences() error = %v", err)
	}
	if strings.Contains(cfg.Backups[0].Name, "/") {
		t.Errorf("Backup name should not contain '/', got %q", cfg.Backups[0].Name)
	}
	if cfg.Backups[0].Name == "../../evil" {
		t.Errorf("Backup name should be sanitized, got %q", cfg.Backups[0].Name)
	}
}
