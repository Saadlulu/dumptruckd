package dump

import (
	"context"
	"fmt"

	"github.com/dumptruckd/dumptruckd/pkg/config"
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
	case "mongodb":
		return nil, fmt.Errorf("mongodb dumper not yet implemented")
	case "sqlite":
		return nil, fmt.Errorf("sqlite dumper not yet implemented")
	case "redis":
		return nil, fmt.Errorf("redis dumper not yet implemented")
	default:
		return nil, fmt.Errorf("unknown database type: %s", cfg.Type)
	}
}

