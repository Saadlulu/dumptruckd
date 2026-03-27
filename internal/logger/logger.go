package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/dumptruckd/dumptruckd/pkg/config"
)

// New creates a configured *slog.Logger from LoggingConfig.
func New(cfg config.LoggingConfig) (*slog.Logger, error) {
	level := parseLevel(cfg.Level)

	writer, err := getWriter(cfg.Output)
	if err != nil {
		return nil, fmt.Errorf("configure log output: %w", err)
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	default:
		handler = slog.NewTextHandler(writer, opts)
	}

	return slog.New(handler), nil
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getWriter(output string) (io.Writer, error) {
	switch strings.ToLower(output) {
	case "", "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("open log file %s: %w", output, err)
		}
		return f, nil
	}
}
