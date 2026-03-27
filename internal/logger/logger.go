package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

// Logger wraps slog.Logger and holds a reference to the log file (if any) for cleanup.
type Logger struct {
	*slog.Logger
	closer io.Closer // nil when output is stdout/stderr
}

// Close releases the underlying log file, if any.
func (l *Logger) Close() error {
	if l.closer != nil {
		return l.closer.Close()
	}
	return nil
}

// New creates a configured Logger from LoggingConfig.
// The caller should call Close() on shutdown to release file handles.
func New(cfg config.LoggingConfig) (*Logger, error) {
	level := parseLevel(cfg.Level)

	writer, closer, err := getWriter(cfg.Output)
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

	return &Logger{
		Logger: slog.New(handler),
		closer: closer,
	}, nil
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

// getWriter returns the writer and an optional closer (non-nil only for file output).
func getWriter(output string) (io.Writer, io.Closer, error) {
	switch strings.ToLower(output) {
	case "", "stdout":
		return os.Stdout, nil, nil
	case "stderr":
		return os.Stderr, nil, nil
	default:
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file %s: %w", output, err)
		}
		return f, f, nil
	}
}
