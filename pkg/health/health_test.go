package health

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNew(t *testing.T) {
	s := New(8080, nil)
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.port != 8080 {
		t.Errorf("port = %d, want 8080", s.port)
	}
}

func TestRecordSuccess(t *testing.T) {
	s := New(8080, discardLogger())
	s.RecordSuccess("test-backup", 5*time.Second)

	bs := s.backups["test-backup"]
	if bs == nil {
		t.Fatal("Backup status not recorded")
	}
	if bs.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", bs.RunCount)
	}
	if bs.FailCount != 0 {
		t.Errorf("FailCount = %d, want 0", bs.FailCount)
	}
	if bs.LastSuccess == nil {
		t.Error("LastSuccess should be set")
	}
	if bs.Duration != "5s" {
		t.Errorf("Duration = %q, want %q", bs.Duration, "5s")
	}
}

func TestRecordFailure(t *testing.T) {
	s := New(8080, discardLogger())
	s.RecordFailure("test-backup", fmt.Errorf("connection refused"))

	bs := s.backups["test-backup"]
	if bs == nil {
		t.Fatal("Backup status not recorded")
	}
	if bs.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", bs.RunCount)
	}
	if bs.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", bs.FailCount)
	}
	if bs.LastFailure == nil {
		t.Error("LastFailure should be set")
	}
	if bs.LastError != "connection refused" {
		t.Errorf("LastError = %q, want %q", bs.LastError, "connection refused")
	}
}

func TestRecordMultiple(t *testing.T) {
	s := New(8080, discardLogger())
	s.RecordSuccess("db1", time.Second)
	s.RecordSuccess("db1", 2*time.Second)
	s.RecordFailure("db1", fmt.Errorf("err"))

	bs := s.backups["db1"]
	if bs.RunCount != 3 {
		t.Errorf("RunCount = %d, want 3", bs.RunCount)
	}
	if bs.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", bs.FailCount)
	}
}

func TestHandleHealth(t *testing.T) {
	s := New(8080, discardLogger())
	s.RecordSuccess("test", time.Second)

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", resp.Header.Get("Content-Type"))
	}

	var status Status
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	if status.Status != "ok" {
		t.Errorf("Status = %q, want %q", status.Status, "ok")
	}
	if _, ok := status.Backups["test"]; !ok {
		t.Error("Backup 'test' should be in response")
	}
}

func TestHandleMetrics(t *testing.T) {
	s := New(8080, discardLogger())
	s.RecordSuccess("prod-db", time.Second)
	s.RecordFailure("prod-db", fmt.Errorf("err"))

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	s.handleMetrics(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	if !strings.Contains(content, "dumptruckd_up 1") {
		t.Error("Metrics should contain dumptruckd_up")
	}
	if !strings.Contains(content, `dumptruckd_backup_runs_total{backup="prod-db"} 2`) {
		t.Errorf("Metrics should show 2 runs, got:\n%s", content)
	}
	if !strings.Contains(content, `dumptruckd_backup_failures_total{backup="prod-db"} 1`) {
		t.Errorf("Metrics should show 1 failure, got:\n%s", content)
	}
}

func TestHandleHealth_NoBackups(t *testing.T) {
	s := New(8080, discardLogger())

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	var status Status
	body, _ := io.ReadAll(w.Result().Body)
	_ = json.Unmarshal(body, &status)

	if status.Status != "ok" {
		t.Errorf("Status = %q, want %q", status.Status, "ok")
	}
}
