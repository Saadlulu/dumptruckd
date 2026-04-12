package dump

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// progressMonitor polls a file's size at a fixed interval and logs growth.
// It runs until the context is cancelled or the done channel is closed.
// This gives users visibility into long-running pg_dump/mysqldump operations
// that otherwise produce no output for minutes.
type progressMonitor struct {
	filePath string
	logger   *slog.Logger
	database string
	tool     string
	interval time.Duration
}

func newProgressMonitor(filePath string, logger *slog.Logger, database string, tool string) *progressMonitor {
	return &progressMonitor{
		filePath: filePath,
		logger:   logger,
		database: database,
		tool:     tool,
		interval: 5 * time.Second,
	}
}

// start begins polling in a goroutine. Returns a function to call when the
// dump is complete (stops the monitor).
func (p *progressMonitor) start(ctx context.Context) func() {
	done := make(chan struct{})
	go p.run(ctx, done)
	return func() { close(done) }
}

func (p *progressMonitor) run(ctx context.Context, done <-chan struct{}) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	start := time.Now()

	for {
		select {
		case <-done:
			// Final size report
			p.logSize(start)
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.logSize(start)
		}
	}
}

func (p *progressMonitor) logSize(start time.Time) {
	info, err := os.Stat(p.filePath)
	if err != nil {
		return // file may not exist yet or was cleaned up
	}

	size := info.Size()
	elapsed := time.Since(start).Round(time.Second)

	p.logger.Info(fmt.Sprintf("%s in progress", p.tool),
		"database", p.database,
		"size", formatBytes(size),
		"elapsed", elapsed,
	)
}

// formatBytes returns a human-readable byte size.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
