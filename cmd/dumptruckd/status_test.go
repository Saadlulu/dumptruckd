package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Saadlulu/dumptruckd/pkg/config"
	"github.com/Saadlulu/dumptruckd/pkg/health"
)

func TestFormatBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		b    int64
		want string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"kilobytes", 1536, "1.5 KB"},
		{"megabytes", 5 * 1024 * 1024, "5.0 MB"},
		{"gigabytes", 3 * 1024 * 1024 * 1024, "3.0 GB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatBytes(tt.b)
			if got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.b, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"seconds", 30 * time.Second, "30s"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours and minutes", 2*time.Hour + 15*time.Minute, "2h 15m"},
		{"days", 3*24*time.Hour + 5*time.Hour + 10*time.Minute, "3d 5h 10m"},
		{"zero", 0, "0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestRetentionSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  config.RetentionConfig
		want string
	}{
		{"none", config.RetentionConfig{}, "none"},
		{"days only", config.RetentionConfig{Days: 7}, "7 days"},
		{"keep_last only", config.RetentionConfig{KeepLast: 5}, "keep last 5"},
		{"both", config.RetentionConfig{Days: 30, KeepLast: 10}, "30 days + keep last 10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := retentionSummary(tt.cfg)
			if got != tt.want {
				t.Errorf("retentionSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUploadSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  config.UploadConfig
		want string
	}{
		{"local default", config.UploadConfig{Type: "local"}, "/var/backups/dumptruckd"},
		{"local custom", config.UploadConfig{Type: "local", Path: "/data/backups"}, "/data/backups"},
		{"s3 basic", config.UploadConfig{Type: "s3", S3: config.S3Config{Bucket: "my-bucket"}}, "s3://my-bucket"},
		{"s3 with prefix and region", config.UploadConfig{Type: "s3", S3: config.S3Config{Bucket: "my-bucket", Prefix: "db", Region: "us-west-2"}}, "s3://my-bucket/db (us-west-2)"},
		{"unknown", config.UploadConfig{Type: "sftp"}, "sftp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := uploadSummary(tt.cfg)
			if got != tt.want {
				t.Errorf("uploadSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestScanLocalBackups(t *testing.T) {
	t.Parallel()

	t.Run("nonexistent directory returns nil", func(t *testing.T) {
		t.Parallel()
		info := scanLocalBackups("/nonexistent/path", "test-backup")
		if info != nil {
			t.Error("expected nil for nonexistent directory")
		}
	})

	t.Run("empty directory returns nil", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		backupDir := filepath.Join(tmpDir, "my-backup")
		if err := os.MkdirAll(backupDir, 0750); err != nil {
			t.Fatal(err)
		}
		info := scanLocalBackups(tmpDir, "my-backup")
		if info != nil {
			t.Error("expected nil for empty directory")
		}
	})

	t.Run("finds backup files", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		backupDir := filepath.Join(tmpDir, "my-backup", "2026", "04", "14")
		if err := os.MkdirAll(backupDir, 0750); err != nil {
			t.Fatal(err)
		}

		// Create two fake backup files
		f1 := filepath.Join(backupDir, "dump_20260414_020000.sql.gz")
		if err := os.WriteFile(f1, make([]byte, 1024), 0640); err != nil {
			t.Fatal(err)
		}
		// Make the second file slightly newer
		time.Sleep(10 * time.Millisecond)
		f2 := filepath.Join(backupDir, "dump_20260414_030000.sql.gz")
		if err := os.WriteFile(f2, make([]byte, 2048), 0640); err != nil {
			t.Fatal(err)
		}

		info := scanLocalBackups(tmpDir, "my-backup")
		if info == nil {
			t.Fatal("expected non-nil info")
		}
		if info.fileCount != 2 {
			t.Errorf("fileCount = %d, want 2", info.fileCount)
		}
		if info.totalBytes != 3072 {
			t.Errorf("totalBytes = %d, want 3072", info.totalBytes)
		}
		if info.latestFile != "dump_20260414_030000.sql.gz" {
			t.Errorf("latestFile = %q, want dump_20260414_030000.sql.gz", info.latestFile)
		}
		if info.latestSize != 2048 {
			t.Errorf("latestSize = %d, want 2048", info.latestSize)
		}
	})
}

func TestFetchHealthStatus_DaemonNotRunning(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Health: config.HealthConfig{
			Enabled: true,
			Port:    0, // will use a port that nothing is listening on
		},
	}
	// Use a random high port that's almost certainly not in use
	cfg.Health.Port = 59999
	status, running := fetchHealthStatus(cfg)
	if running {
		t.Error("expected running=false when no server is listening")
	}
	if status != nil {
		t.Error("expected nil status when no server is listening")
	}
}

func TestFetchHealthStatus_HealthDisabled(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Health: config.HealthConfig{Enabled: false},
	}
	status, running := fetchHealthStatus(cfg)
	if running {
		t.Error("expected running=false when health is disabled")
	}
	if status != nil {
		t.Error("expected nil status when health is disabled")
	}
}

func TestFetchHealthStatus_Success(t *testing.T) {
	now := time.Now()
	healthResp := health.Status{
		Status:    "ok",
		Uptime:    "1h30m0s",
		StartedAt: now.Add(-90 * time.Minute),
		Backups: map[string]health.BackupStatus{
			"prod-db": {
				LastSuccess:         &now,
				RunCount:            10,
				LastBackupSizeBytes: 1024 * 1024,
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(healthResp)
	}))
	defer srv.Close()

	// Extract port from test server
	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)

	cfg := &config.Config{
		Health: config.HealthConfig{
			Enabled: true,
			Port:    port,
		},
	}

	status, running := fetchHealthStatus(cfg)
	if !running {
		t.Fatal("expected running=true")
	}
	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.Status != "ok" {
		t.Errorf("status = %q, want %q", status.Status, "ok")
	}
	if len(status.Backups) != 1 {
		t.Errorf("backup count = %d, want 1", len(status.Backups))
	}
	if bs, ok := status.Backups["prod-db"]; ok {
		if bs.RunCount != 10 {
			t.Errorf("RunCount = %d, want 10", bs.RunCount)
		}
	} else {
		t.Error("expected prod-db in backups")
	}
}

func TestPrintStatusJSON(t *testing.T) {
	cfg := &config.Config{
		Backups: []config.BackupConfig{
			{
				Name:     "test-db",
				Schedule: "0 0 2 * * *",
				Database: config.DatabaseConfig{Type: "postgres", Database: "mydb"},
				Upload:   config.UploadConfig{Type: "local", Path: "/nonexistent"},
				Retention: config.RetentionConfig{Days: 7, KeepLast: 3},
			},
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printStatusJSON(cfg, nil, false)

	w.Close()
	os.Stdout = old

	// Read from the pipe
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	var result statusJSON
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, output)
	}

	if result.DaemonRunning {
		t.Error("expected daemon_running=false")
	}
	if len(result.Backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(result.Backups))
	}
	if result.Backups[0].Name != "test-db" {
		t.Errorf("backup name = %q, want %q", result.Backups[0].Name, "test-db")
	}
	if result.Backups[0].Retention != "7 days + keep last 3" {
		t.Errorf("retention = %q, want %q", result.Backups[0].Retention, "7 days + keep last 3")
	}
}
