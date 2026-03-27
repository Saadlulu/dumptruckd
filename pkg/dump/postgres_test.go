package dump

import (
	"context"
	"os"
	"testing"

	"github.com/dumptruckd/dumptruckd/pkg/config"
)

func TestNewPostgresDumper_MissingHost(t *testing.T) {
	_, err := NewPostgresDumper(config.DatabaseConfig{
		Type:     "postgres",
		Database: "testdb",
		Username: "user",
	})
	if err == nil {
		t.Error("NewPostgresDumper() should error when host is missing")
	}
}

func TestNewPostgresDumper_MissingDatabase(t *testing.T) {
	_, err := NewPostgresDumper(config.DatabaseConfig{
		Type:     "postgres",
		Host:     "localhost",
		Username: "user",
	})
	if err == nil {
		t.Error("NewPostgresDumper() should error when database is missing")
	}
}

func TestNewPostgresDumper_ValidConfig(t *testing.T) {
	dumper, err := NewPostgresDumper(config.DatabaseConfig{
		Type:     "postgres",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})
	if err != nil {
		t.Fatalf("NewPostgresDumper() unexpected error: %v", err)
	}
	if dumper == nil {
		t.Error("NewPostgresDumper() returned nil")
	}
}

func TestNewPostgresDumper_DefaultPort(t *testing.T) {
	dumper, err := NewPostgresDumper(config.DatabaseConfig{
		Type:     "postgres",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
		// Port intentionally omitted — should default to 5432
	})
	if err != nil {
		t.Fatalf("NewPostgresDumper() unexpected error: %v", err)
	}
	if dumper.cfg.Port != 0 {
		t.Errorf("Port should be 0 (default applied at dump time), got %d", dumper.cfg.Port)
	}
}

func TestPostgresDumper_Dump_MissingPassword(t *testing.T) {
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_PASSWORD_testdb")

	dumper, _ := NewPostgresDumper(config.DatabaseConfig{
		Type:     "postgres",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})

	_, err := dumper.Dump(context.Background())
	if err == nil {
		t.Error("Dump() should error when DB_PASSWORD is not set")
	}
}

func TestPostgresDumper_TestDump_MissingPassword(t *testing.T) {
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_PASSWORD_testdb")

	dumper, _ := NewPostgresDumper(config.DatabaseConfig{
		Type:     "postgres",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})

	_, err := dumper.TestDump(context.Background())
	if err == nil {
		t.Error("TestDump() should error when DB_PASSWORD is not set")
	}
}
