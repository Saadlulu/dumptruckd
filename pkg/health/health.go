package health

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Saadlulu/dumptruckd/internal/utils"
)

// DefaultPort is the default health server port.
const DefaultPort = 8080

// Status represents the health status response.
type Status struct {
	Status    string                  `json:"status"`
	Uptime    string                  `json:"uptime"`
	StartedAt time.Time               `json:"started_at"`
	Backups   map[string]BackupStatus `json:"backups,omitempty"`
}

// BackupStatus tracks the last run info for a backup job.
type BackupStatus struct {
	LastRun             *time.Time `json:"last_run,omitempty"`
	LastSuccess         *time.Time `json:"last_success,omitempty"`
	LastFailure         *time.Time `json:"last_failure,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
	Duration            string     `json:"duration,omitempty"`
	RunCount            int64      `json:"run_count"`
	FailCount           int64      `json:"fail_count"`
	LastBackupSizeBytes int64      `json:"last_backup_size_bytes,omitempty"`
	NextScheduledRun    *time.Time `json:"next_scheduled_run,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
}

// Server provides health check and metrics endpoints.
type Server struct {
	port        int
	log         *slog.Logger
	startedAt   time.Time
	mu          sync.RWMutex
	backups     map[string]*BackupStatus
	server      *http.Server
	bearerToken string // optional; if set, requests must include Authorization: Bearer <token>
}

// New creates a new health server on the given port.
// If port is 0, DefaultPort (8080) is used.
// Bearer token can be set via config or HEALTH_BEARER_TOKEN env var (env var takes precedence).
func New(port int, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	if port == 0 {
		port = DefaultPort
	}
	return &Server{
		port:        port,
		log:         log,
		startedAt:   utils.Now(),
		backups:     make(map[string]*BackupStatus),
		bearerToken: os.Getenv("HEALTH_BEARER_TOKEN"),
	}
}

// WithToken sets the bearer token for endpoint authentication.
// Overrides the HEALTH_BEARER_TOKEN env var if both are set.
func (s *Server) WithToken(token string) *Server {
	if token != "" {
		s.bearerToken = token
	}
	return s
}

// RecordSuccess records a successful backup run and resets consecutive failures.
func (s *Server) RecordSuccess(name string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs := s.getOrCreate(name)
	now := utils.Now()
	bs.LastRun = &now
	bs.LastSuccess = &now
	bs.Duration = duration.String()
	bs.RunCount++
	bs.ConsecutiveFailures = 0
}

// RecordFailure records a failed backup run and increments consecutive failures.
func (s *Server) RecordFailure(name string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs := s.getOrCreate(name)
	now := utils.Now()
	bs.LastRun = &now
	bs.LastFailure = &now
	bs.LastError = err.Error()
	bs.RunCount++
	bs.FailCount++
	bs.ConsecutiveFailures++
}

// RecordSize records the size of the last backup for a job.
func (s *Server) RecordSize(name string, bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs := s.getOrCreate(name)
	bs.LastBackupSizeBytes = bytes
}

// SetNextRun sets the next scheduled run time for a backup job.
func (s *Server) SetNextRun(name string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs := s.getOrCreate(name)
	bs.NextScheduledRun = &t
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
	mux.HandleFunc("/healthz", s.requireAuth(s.handleHealth))
	mux.HandleFunc("/metrics", s.requireAuth(s.handleMetrics))

	s.server = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		s.log.Info("health server started", "port", s.port, "auth", s.bearerToken != "")
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

// requireAuth wraps a handler with optional bearer token authentication.
// If no token is configured, requests pass through.
// Uses constant-time comparison to prevent timing attacks.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.bearerToken != "" {
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + s.bearerToken
			if len(auth) != len(expected) || subtle.ConstantTimeCompare([]byte(auth), []byte(expected)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := Status{
		Status:    "ok",
		Uptime:    utils.Now().Sub(s.startedAt).Round(time.Second).String(),
		StartedAt: s.startedAt,
		Backups:   make(map[string]BackupStatus),
	}

	for name, bs := range s.backups {
		status.Backups[name] = *bs
		if bs.ConsecutiveFailures >= 3 {
			status.Status = "degraded"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		s.log.Error("failed to encode health response", "error", err)
	}
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
	fmt.Fprintf(w, "dumptruckd_uptime_seconds %.0f\n\n", utils.Now().Sub(s.startedAt).Seconds())

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
