package dump

import (
	"context"
	"os"
	"testing"

	"github.com/dumptruckd/dumptruckd/pkg/config"
)

func TestNewMySQLDumper_MissingHost(t *testing.T) {
	_, err := NewMySQLDumper(config.DatabaseConfig{
		Type:     "mysql",
		Database: "testdb",
		Username: "user",
	})
	if err == nil {
		t.Error("NewMySQLDumper() should error when host is missing")
	}
}

func TestNewMySQLDumper_MissingDatabase(t *testing.T) {
	_, err := NewMySQLDumper(config.DatabaseConfig{
		Type:     "mysql",
		Host:     "localhost",
		Username: "user",
	})
	if err == nil {
		t.Error("NewMySQLDumper() should error when database is missing")
	}
}

func TestNewMySQLDumper_ValidConfig(t *testing.T) {
	dumper, err := NewMySQLDumper(config.DatabaseConfig{
		Type:     "mysql",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})
	if err != nil {
		t.Fatalf("NewMySQLDumper() unexpected error: %v", err)
	}
	if dumper == nil {
		t.Error("NewMySQLDumper() returned nil")
	}
}

func TestNewMySQLDumper_DefaultPort(t *testing.T) {
	dumper, err := NewMySQLDumper(config.DatabaseConfig{
		Type:     "mysql",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Port 0 means default (3306) applied at dump time
	if dumper.cfg.Port != 0 {
		t.Errorf("Port should be 0 (default applied at dump time), got %d", dumper.cfg.Port)
	}
}

func TestMySQLDumper_Dump_MissingPassword(t *testing.T) {
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_PASSWORD_testdb")

	dumper, _ := NewMySQLDumper(config.DatabaseConfig{
		Type:     "mysql",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})

	_, err := dumper.Dump(context.Background())
	if err == nil {
		t.Error("Dump() should error when DB_PASSWORD is not set")
	}
}

func TestMySQLDumper_TestDump_MissingPassword(t *testing.T) {
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_PASSWORD_testdb")

	dumper, _ := NewMySQLDumper(config.DatabaseConfig{
		Type:     "mysql",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})

	_, err := dumper.TestDump(context.Background())
	if err == nil {
		t.Error("TestDump() should error when DB_PASSWORD is not set")
	}
}

func TestNewDumper_MySQL(t *testing.T) {
	dumper, err := NewDumper(config.DatabaseConfig{
		Type:     "mysql",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})
	if err != nil {
		t.Fatalf("NewDumper() should support mysql, got error: %v", err)
	}
	if dumper == nil {
		t.Error("NewDumper() returned nil for mysql")
	}
}
