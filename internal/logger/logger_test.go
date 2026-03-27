package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

func TestNew_DefaultConfig(t *testing.T) {
	l, err := New(config.LoggingConfig{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if l == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_JSONFormat(t *testing.T) {
	l, err := New(config.LoggingConfig{Format: "json", Level: "info"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if l == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_TextFormat(t *testing.T) {
	l, err := New(config.LoggingConfig{Format: "text", Level: "debug"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if l == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_FileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	l, err := New(config.LoggingConfig{Output: logFile, Level: "info", Format: "text"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	l.Info("test message")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	if len(data) == 0 {
		t.Error("Log file should contain output")
	}
}

func TestNew_InvalidFileOutput(t *testing.T) {
	_, err := New(config.LoggingConfig{Output: "/nonexistent/dir/test.log"})
	if err == nil {
		t.Error("New() should error for invalid file path")
	}
}

func TestNew_StderrOutput(t *testing.T) {
	l, err := New(config.LoggingConfig{Output: "stderr"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if l == nil {
		t.Fatal("New() returned nil")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
