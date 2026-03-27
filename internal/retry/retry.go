package retry

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"
)

// Config holds retry settings.
type Config struct {
	MaxRetries int
	BaseDelay  time.Duration
}

// DefaultConfig returns sensible retry defaults.
func DefaultConfig() Config {
	return Config{
		MaxRetries: 3,
		BaseDelay:  2 * time.Second,
	}
}

// Do executes fn with exponential backoff retries.
// Returns the result of the last attempt if all retries fail.
func Do(ctx context.Context, cfg Config, log *slog.Logger, operation string, fn func() error) error {
	if cfg.MaxRetries <= 0 {
		return fn()
	}

	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if attempt < cfg.MaxRetries {
			delay := time.Duration(math.Pow(2, float64(attempt))) * cfg.BaseDelay
			if log != nil {
				log.Warn("operation failed, retrying",
					"operation", operation,
					"attempt", attempt+1,
					"max_retries", cfg.MaxRetries,
					"delay", delay,
					"error", lastErr,
				)
			}

			select {
			case <-ctx.Done():
				return fmt.Errorf("%s cancelled during retry: %w", operation, ctx.Err())
			case <-time.After(delay):
			}
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", operation, cfg.MaxRetries+1, lastErr)
}
