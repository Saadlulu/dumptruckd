package health

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status response.
type Status struct {
	Status    string            `json:"status"`
	Uptime    string            `json:"uptime"`
	StartedAt time.Time        `json:"started_at"`
	Backups   map[string]BackupStatus `json:"backups,omitempty"`
}

// BackupStatus tracks the last run info for a backup job.
type BackupStatus struct {
	LastRun     *time.Time `json:"last_run,omitempty"`
	LastSuccess *time.Time `json:"last_success,omitempty"`
	LastFailure *time.Time `json:"last_failure,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	Duration    string     `json:"duration,omitempty"`
	RunCount    int64      `json:"run_count"`
	FailCount   int64      `json:"fail_count"`
}

// Server provides health check and metrics endpoints.
type Server struct {
	port      int
	log       *slog.Logger
	startedAt time.Time
	mu        sync.RWMutex
	backups   map[string]*BackupStatus
	server    *http.Server
}

// New creates a new health server on the given port.
func New(port int, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		port:      port,
		log:       log,
		startedAt: time.Now(),
		backups:   make(map[string]*BackupStatus),
	}
}

// RecordSuccess records a successful backup run.
func (s *Server) RecordSuccess(name string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs := s.getOrCreate(name)
	now := time.Now()
	bs.LastRun = &now
	bs.LastSuccess = &now
	bs.Duration = duration.String()
	bs.RunCount++
}

// RecordFailure records a failed backup run.
func (s *Server) RecordFailure(name string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs := s.getOrCreate(name)
	now := time.Now()
	bs.LastRun = &now
	bs.LastFailure = &now
	bs.LastError = err.Error()
	bs.RunCount++
	bs.FailCount++
}

func (s *Server) getOrCreate(name string) *BackupStatus {
	bs, ok := s.backups[name]
	if !ok {
		bs = &BackupStatus{}
		s.backups[name] = bs
	}
	return bs
}

// Start begins serving health endpoints. Non-blocking.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/metrics", s.handleMetrics)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		s.log.Info("health server started", "port", s.port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("health server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the health server.
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := Status{
		Status:    "ok",
		Uptime:    time.Since(s.startedAt).Round(time.Second).String(),
		StartedAt: s.startedAt,
		Backups:   make(map[string]BackupStatus),
	}

	for name, bs := range s.backups {
		status.Backups[name] = *bs
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain")

	fmt.Fprintf(w, "# HELP dumptruckd_up Whether the daemon is running\n")
	fmt.Fprintf(w, "# TYPE dumptruckd_up gauge\n")
	fmt.Fprintf(w, "dumptruckd_up 1\n\n")

	fmt.Fprintf(w, "# HELP dumptruckd_uptime_seconds Uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE dumptruckd_uptime_seconds gauge\n")
	fmt.Fprintf(w, "dumptruckd_uptime_seconds %.0f\n\n", time.Since(s.startedAt).Seconds())

	fmt.Fprintf(w, "# HELP dumptruckd_backup_runs_total Total backup runs\n")
	fmt.Fprintf(w, "# TYPE dumptruckd_backup_runs_total counter\n")
	for name, bs := range s.backups {
		fmt.Fprintf(w, "dumptruckd_backup_runs_total{backup=%q} %d\n", name, bs.RunCount)
	}

	fmt.Fprintf(w, "\n# HELP dumptruckd_backup_failures_total Total backup failures\n")
	fmt.Fprintf(w, "# TYPE dumptruckd_backup_failures_total counter\n")
	for name, bs := range s.backups {
		fmt.Fprintf(w, "dumptruckd_backup_failures_total{backup=%q} %d\n", name, bs.FailCount)
	}
}
