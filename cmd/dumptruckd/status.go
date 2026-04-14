package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Saadlulu/dumptruckd/pkg/config"
	"github.com/Saadlulu/dumptruckd/pkg/health"
)

// Box-drawing characters for consistent output.
const (
	boxTopLeft     = "\u250C" // ┌
	boxTopRight    = "\u2510" // ┐
	boxBottomLeft  = "\u2514" // └
	boxBottomRight = "\u2518" // ┘
	boxHorizontal  = "\u2500" // ─
	boxVertical    = "\u2502" // │
	boxTeeRight    = "\u251C" // ├
	boxTeeLeft     = "\u2524" // ┤
)

// localBackupInfo holds filesystem-derived info about a backup's stored files.
type localBackupInfo struct {
	fileCount  int
	totalBytes int64
	latestFile string
	latestTime time.Time
	latestSize int64
	oldestTime time.Time
}

// runStatusSubcommand implements the "status" subcommand.
func runStatusSubcommand() {
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
	configPath := statusCmd.String("config", "", "Path to configuration file")
	jsonOutput := statusCmd.Bool("json", false, "Output status as JSON")
	if err := statusCmd.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = findConfig()
	}

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load config: %v\n", err)
		os.Exit(1)
	}

	liveStatus, daemonRunning := fetchHealthStatus(cfg)

	if *jsonOutput {
		printStatusJSON(cfg, liveStatus, daemonRunning)
	} else {
		printStatusText(cfg, liveStatus, daemonRunning)
	}
}

// ─── Health endpoint client ──────────────────────────────────────────────────

// fetchHealthStatus attempts to connect to the running daemon's health endpoint.
// The health server binds to 127.0.0.1 only — no sensitive data leaves the host.
func fetchHealthStatus(cfg *config.Config) (*health.Status, bool) {
	if !cfg.Health.Enabled {
		return nil, false
	}

	port := cfg.Health.Port
	if port == 0 {
		port = health.DefaultPort
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", port)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false
	}

	token := os.Getenv("HEALTH_BEARER_TOKEN")
	if cfg.Health.Token != "" {
		token = cfg.Health.Token
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}

	var status health.Status
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, false
	}

	return &status, true
}

// ─── Filesystem scanner ─────────────────────────────────────────────────────

// scanLocalBackups walks the local backup directory for a backup job and returns file info.
func scanLocalBackups(basePath string, backupName string) *localBackupInfo {
	dir := filepath.Join(basePath, backupName)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	result := &localBackupInfo{}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		finfo, err := d.Info()
		if err != nil {
			return nil
		}

		result.fileCount++
		result.totalBytes += finfo.Size()

		if result.fileCount == 1 || finfo.ModTime().After(result.latestTime) {
			result.latestTime = finfo.ModTime()
			result.latestFile = filepath.Base(path)
			result.latestSize = finfo.Size()
		}
		if result.fileCount == 1 || finfo.ModTime().Before(result.oldestTime) {
			result.oldestTime = finfo.ModTime()
		}
		return nil
	})

	if result.fileCount == 0 {
		return nil
	}
	return result
}

// ─── Formatting helpers ─────────────────────────────────────────────────────

const boxWidth = 66

// hLine returns a full-width horizontal line: ├──...──┤ or ┌──...──┐ etc.
func hLine(left, right string) string {
	return left + strings.Repeat(boxHorizontal, boxWidth-2) + right
}

// row returns a box row with content padded to width.
func row(content string) string {
	visible := []rune(content)
	inner := boxWidth - 4 // space for "│  " and "│"
	if len(visible) > inner {
		// Truncate with ellipsis to keep the box intact
		visible = append(visible[:inner-3], '.', '.', '.')
	}
	pad := inner - len(visible)
	if pad < 0 {
		pad = 0
	}
	return boxVertical + "  " + string(visible) + strings.Repeat(" ", pad) + boxVertical
}

// rowKV returns a key-value row with aligned columns.
func rowKV(key, value string) string {
	return row(fmt.Sprintf("%-14s %s", key, value))
}

// sectionHeader returns a section divider with a label.
func sectionHeader(label string) string {
	labelLen := len([]rune(label))
	remaining := boxWidth - 5 - labelLen // 5 = "├── " + "┤" overhead
	if remaining < 1 {
		remaining = 1
	}
	return boxTeeRight + boxHorizontal + " " + label + " " + strings.Repeat(boxHorizontal, remaining) + boxTeeLeft
}

// emptyRow returns an empty padded row.
func emptyRow() string {
	return boxVertical + strings.Repeat(" ", boxWidth-2) + boxVertical
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

// formatDuration returns a human-friendly duration string like "5d 3h 22m".
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	if len(parts) == 0 {
		return "0m"
	}
	return strings.Join(parts, " ")
}

// retentionSummary returns a human-readable retention policy description.
func retentionSummary(r config.RetentionConfig) string {
	var parts []string
	if r.Days > 0 {
		parts = append(parts, fmt.Sprintf("%d days", r.Days))
	}
	if r.KeepLast > 0 {
		parts = append(parts, fmt.Sprintf("keep last %d", r.KeepLast))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " + ")
}

// uploadSummary returns a short description of the upload destination.
// Intentionally omits credentials and sensitive endpoint details.
func uploadSummary(u config.UploadConfig) string {
	switch u.Type {
	case "s3":
		s := "s3://" + u.S3.Bucket
		if u.S3.Prefix != "" {
			s += "/" + u.S3.Prefix
		}
		if u.S3.Region != "" {
			s += " (" + u.S3.Region + ")"
		}
		return s
	case "local":
		p := u.Path
		if p == "" {
			p = "/var/backups/dumptruckd"
		}
		return p
	default:
		return u.Type
	}
}

// ─── Text output ────────────────────────────────────────────────────────────

// printStatusText prints a beautifully formatted status report to stdout.
func printStatusText(cfg *config.Config, live *health.Status, running bool) {
	fmt.Println()
	fmt.Println(hLine(boxTopLeft, boxTopRight))
	fmt.Println(row("dumptruckd"))
	fmt.Println(sectionHeader("Daemon"))

	if running && live != nil {
		status := "RUNNING"
		if live.Status == "degraded" {
			status = "DEGRADED"
		}
		fmt.Println(rowKV("Status", status))
		fmt.Println(rowKV("Uptime", live.Uptime))
		fmt.Println(rowKV("Since", live.StartedAt.Local().Format("2006-01-02 15:04:05")))
	} else if cfg.Health.Enabled {
		fmt.Println(rowKV("Status", "NOT RUNNING"))
	} else {
		fmt.Println(rowKV("Status", "UNKNOWN (health endpoint disabled)"))
	}

	fmt.Println(rowKV("Jobs", fmt.Sprintf("%d configured", len(cfg.Backups))))

	for i, backup := range cfg.Backups {
		_ = i
		fmt.Println(sectionHeader(backup.Name))

		// Config info — only database name and type, no host/user/port
		fmt.Println(rowKV("Database", fmt.Sprintf("%s (%s)", backup.Database.Database, backup.Database.Type)))
		fmt.Println(rowKV("Schedule", backup.Schedule))
		fmt.Println(rowKV("Upload", uploadSummary(backup.Upload)))
		fmt.Println(rowKV("Retention", retentionSummary(backup.Retention)))

		// Live metrics from health endpoint
		if running && live != nil {
			if bs, ok := live.Backups[backup.Name]; ok {
				printBackupLiveRows(bs)
			} else {
				fmt.Println(emptyRow())
				fmt.Println(rowKV("Runs", "no data yet"))
			}
		}

		// Filesystem info for local uploads
		if backup.Upload.Type == "local" {
			basePath := backup.Upload.Path
			if basePath == "" {
				basePath = "/var/backups/dumptruckd"
			}
			if info := scanLocalBackups(basePath, backup.Name); info != nil {
				fmt.Println(emptyRow())
				fmt.Println(rowKV("On Disk", fmt.Sprintf("%d files, %s total", info.fileCount, formatBytes(info.totalBytes))))
				fmt.Println(rowKV("Latest File", info.latestFile))
				fmt.Println(rowKV("Latest Size", formatBytes(info.latestSize)))
				fmt.Println(rowKV("Latest Date", info.latestTime.Local().Format("2006-01-02 15:04:05")))
			} else if !running {
				fmt.Println(emptyRow())
				fmt.Println(rowKV("On Disk", "no backups found"))
			}
		}
	}

	fmt.Println(hLine(boxBottomLeft, boxBottomRight))
	fmt.Println()
}

// printBackupLiveRows prints the live health metrics rows for a single backup.
func printBackupLiveRows(bs health.BackupStatus) {
	fmt.Println(emptyRow())

	// Status line
	if bs.ConsecutiveFailures >= 3 {
		fmt.Println(rowKV("Health", fmt.Sprintf("FAILING (%d consecutive)", bs.ConsecutiveFailures)))
	} else if bs.FailCount > 0 && bs.ConsecutiveFailures > 0 {
		fmt.Println(rowKV("Health", fmt.Sprintf("DEGRADED (%d consecutive)", bs.ConsecutiveFailures)))
	} else {
		fmt.Println(rowKV("Health", "OK"))
	}

	// Last success
	if bs.LastSuccess != nil {
		line := bs.LastSuccess.Local().Format("2006-01-02 15:04:05")
		if bs.Duration != "" {
			line += " (" + bs.Duration + ")"
		}
		fmt.Println(rowKV("Last Success", line))
	}

	// Last failure
	if bs.LastFailure != nil {
		fmt.Println(rowKV("Last Failure", bs.LastFailure.Local().Format("2006-01-02 15:04:05")))
		if bs.LastError != "" {
			// Truncate long errors to fit the box
			errMsg := bs.LastError
			maxLen := boxWidth - 20
			if len(errMsg) > maxLen {
				errMsg = errMsg[:maxLen-3] + "..."
			}
			fmt.Println(rowKV("Error", errMsg))
		}
	}

	// Size
	if bs.LastBackupSizeBytes > 0 {
		fmt.Println(rowKV("Last Size", formatBytes(bs.LastBackupSizeBytes)))
	}

	// Run counts
	fmt.Println(rowKV("Runs", fmt.Sprintf("%d total, %d failed", bs.RunCount, bs.FailCount)))

	// Next run
	if bs.NextScheduledRun != nil {
		until := time.Until(*bs.NextScheduledRun)
		fmt.Println(rowKV("Next Run", fmt.Sprintf("%s (in %s)",
			bs.NextScheduledRun.Local().Format("2006-01-02 15:04:05"),
			formatDuration(until),
		)))
	}
}

// ─── JSON output ────────────────────────────────────────────────────────────

// statusJSON is the structured output for --json mode.
type statusJSON struct {
	DaemonRunning bool               `json:"daemon_running"`
	DaemonStatus  string             `json:"daemon_status,omitempty"`
	Uptime        string             `json:"uptime,omitempty"`
	StartedAt     *time.Time         `json:"started_at,omitempty"`
	Backups       []backupStatusJSON `json:"backups"`
}

// backupStatusJSON is the per-backup JSON output.
type backupStatusJSON struct {
	Name      string               `json:"name"`
	Schedule  string               `json:"schedule"`
	Database  string               `json:"database"`
	DBType    string               `json:"db_type"`
	Upload    string               `json:"upload_type"`
	Retention string               `json:"retention"`
	Live      *health.BackupStatus `json:"live,omitempty"`
	Local     *localBackupJSON     `json:"local,omitempty"`
}

// localBackupJSON is the filesystem scan result for JSON output.
type localBackupJSON struct {
	FileCount  int       `json:"file_count"`
	TotalBytes int64     `json:"total_bytes"`
	LatestFile string    `json:"latest_file"`
	LatestTime time.Time `json:"latest_time"`
	LatestSize int64     `json:"latest_size_bytes"`
}

// printStatusJSON outputs the full status as JSON.
func printStatusJSON(cfg *config.Config, live *health.Status, running bool) {
	out := statusJSON{
		DaemonRunning: running,
	}

	if running && live != nil {
		out.DaemonStatus = live.Status
		out.Uptime = live.Uptime
		out.StartedAt = &live.StartedAt
	}

	for _, backup := range cfg.Backups {
		bs := backupStatusJSON{
			Name:      backup.Name,
			Schedule:  backup.Schedule,
			Database:  backup.Database.Database,
			DBType:    backup.Database.Type,
			Upload:    backup.Upload.Type,
			Retention: retentionSummary(backup.Retention),
		}

		if running && live != nil {
			if lbs, ok := live.Backups[backup.Name]; ok {
				bs.Live = &lbs
			}
		}

		if backup.Upload.Type == "local" {
			basePath := backup.Upload.Path
			if basePath == "" {
				basePath = "/var/backups/dumptruckd"
			}
			if info := scanLocalBackups(basePath, backup.Name); info != nil {
				bs.Local = &localBackupJSON{
					FileCount:  info.fileCount,
					TotalBytes: info.totalBytes,
					LatestFile: info.latestFile,
					LatestTime: info.latestTime,
					LatestSize: info.latestSize,
				}
			}
		}

		out.Backups = append(out.Backups, bs)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}
