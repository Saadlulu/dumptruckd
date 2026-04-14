package dump

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// progressMonitor polls a file's size at a fixed interval and logs growth.
// Gives users visibility into long-running pg_dump/mysqldump operations.
type progressMonitor struct {
	filePath string
	logger   *slog.Logger
	database string
	tool     string
	interval time.Duration
	started  time.Time
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

// start begins polling in a goroutine. Returns a stop function.
// Call stop BEFORE logging completion to avoid a stale progress line.
func (p *progressMonitor) start(ctx context.Context) func() {
	p.started = time.Now()
	done := make(chan struct{})
	go p.run(ctx, done)
	return func() { close(done) }
}

func (p *progressMonitor) run(ctx context.Context, done <-chan struct{}) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.logSize()
		}
	}
}

func (p *progressMonitor) logSize() {
	info, err := os.Stat(p.filePath)
	if err != nil {
		return
	}
	elapsed := time.Since(p.started).Round(time.Second)
	p.logger.Info("      "+formatBytes(info.Size())+" written", "elapsed", elapsed)
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
