package dump

import (
	"context"
	"fmt"

	"github.com/Saadlulu/dumptruckd/pkg/config"
)

// Dumper interface for database dump adapters
type Dumper interface {
	Dump(ctx context.Context) (string, error) // Returns path to dump file
}

// TestDumper extends Dumper with a lightweight test dump for config validation.
type TestDumper interface {
	Dumper
	TestDump(ctx context.Context) (string, error) // Returns path to small test dump file
}

// NewDumper creates a dumper based on database config
func NewDumper(cfg config.DatabaseConfig) (Dumper, error) {
	switch cfg.Type {
	case "postgres":
		return NewPostgresDumper(cfg)
	case "mysql":
		return NewMySQLDumper(cfg)
	default:
		return nil, fmt.Errorf("unknown or unsupported database type: %s", cfg.Type)
	}
}
