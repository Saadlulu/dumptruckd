# dumptruckd

> A modular, production-ready database backup daemon

**dumptruckd** is a clean, extensible backup daemon that handles periodic database dumps, compression, and uploads to various storage backends. Built with Go, designed for reliability and simplicity.

## Features

- 🔌 **Pluggable Architecture**: Modular adapters for databases, compressors, uploaders, and notifiers
- 📅 **Cron-based Scheduling**: Flexible scheduling using cron expressions with concurrency limits
- 🗄️ **Multiple Database Support**: PostgreSQL (with more coming)
- 🗜️ **Compression**: Gzip support (zstd, xz coming)
- ☁️ **Multiple Storage Backends**: S3, local filesystem (more coming)
- 🔔 **Notifications**: Slack, webhooks (email, Discord coming)
- � **Retry with Backoff**: Configurable exponential backoff for transient failures
- 📊 **Health & Metrics**: `/healthz` and `/metrics` (Prometheus format) endpoints
- 📝 **Structured Logging**: JSON or text output via `log/slog`, configurable level and destination
- �🐳 **Docker Ready**: Containerized with minimal dependencies
- 🔒 **Security First**: Credentials via environment variables, no secrets in config
- 🛑 **Graceful Shutdown**: Waits for in-progress backups to complete on SIGTERM

## Quick Start

### Installation

```bash
# From source
go install github.com/dumptruckd/dumptruckd/cmd/dumptruckd@latest

# Or build from source
git clone https://github.com/dumptruckd/dumptruckd
cd dumptruckd
make build
```

### Configuration

dumptruckd supports two configuration approaches:

#### Option 1: Modular Configuration (Recommended)

1. Copy the modular config template:
```bash
cp config/dumptruckd.toml.example config/dumptruckd.toml
```

2. Edit component files in `config/config.d/`:
   - `databases.toml` - Define database connections
   - `uploaders.toml` - Define upload destinations (S3, local, etc.)
   - `compressors.toml` - Define compression methods
   - `retention.toml` - Define retention policies
   - `backups.toml` - Define backup jobs that reference components

3. Set environment variables:
```bash
export DB_PASSWORD="your-db-password"
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
```

#### Option 2: Single File Configuration

```bash
cp config/example-single-file.toml config/dumptruckd.toml
```

Edit with your settings, set environment variables, done.

### Run

```bash
dumptruckd -config config/dumptruckd.toml
```

### Test Configuration

Validate your setup before running backups:

```bash
dumptruckd -test -config config/dumptruckd.toml
```

This tests database connections, compression, uploads (with verify + cleanup), notifications, and runs a full pipeline dry-run for each backup job.

## Configuration Reference

### Logging

```toml
[logging]
level = "info"       # debug, info, warn, error
format = "text"      # text, json
output = "stdout"    # stdout, stderr, or a file path
```

### Health & Metrics

```toml
[health]
enabled = true
port = 8080
```

Endpoints:
- `GET /healthz` — JSON status with per-backup run stats (last run, success/failure counts, duration)
- `GET /metrics` — Prometheus-format metrics (`dumptruckd_up`, `dumptruckd_backup_runs_total`, `dumptruckd_backup_failures_total`)

### Backup Jobs

```toml
[[backup]]
name = "my_backup"
schedule = "0 */6 * * *"
database_ref = "prod_postgres"
compress_ref = "fast"
upload_ref = "prod_s3"
retention_ref = "ten_days"

[backup.notify]
type = "slack"
  [backup.notify.slack]
  webhook_url = "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
```

Components can be referenced by name or defined inline. See `config/example-single-file.toml` for full examples.

### Environment Variables

| Variable | Used By | Required |
|----------|---------|----------|
| `DB_PASSWORD` or `DB_PASSWORD_{DBNAME}` | PostgreSQL dumper | Yes |
| `AWS_ACCESS_KEY_ID` | S3 uploader | For S3 uploads |
| `AWS_SECRET_ACCESS_KEY` | S3 uploader | For S3 uploads |
| `SLACK_WEBHOOK_URL` | Slack notifier | If Slack configured |

## Docker

```bash
docker build -t dumptruckd .

docker run --rm \
  -v $(pwd)/config:/app/config \
  -e AWS_ACCESS_KEY_ID=xxx \
  -e AWS_SECRET_ACCESS_KEY=xxx \
  -e DB_PASSWORD=xxx \
  dumptruckd
```

## Development

```bash
make deps          # Download dependencies
make build         # Build binary
make test          # Run tests
make test-coverage # Tests with coverage report
make lint          # Run linters
make fmt           # Format code
make vet           # Run go vet
```

## Project Structure

```
dumptruckd/
├── cmd/dumptruckd/      # Entry point, CLI flags, graceful shutdown
├── pkg/
│   ├── config/          # TOML config loading, validation, reference resolution
│   ├── scheduler/       # Cron scheduling with concurrency limits and graceful drain
│   ├── dump/            # Database dump adapters (Dumper, TestDumper interfaces)
│   ├── compress/        # Compression adapters (Compressor interface)
│   ├── upload/          # Upload adapters (Uploader, VerifiableUploader interfaces)
│   ├── notify/          # Notification adapters (Notifier interface)
│   ├── retention/       # Local filesystem retention cleanup
│   ├── health/          # Health check and Prometheus metrics server
│   └── test/            # Built-in configuration test framework
├── internal/
│   ├── logger/          # Structured logging (log/slog) setup
│   ├── retry/           # Exponential backoff retry logic
│   └── utils/           # Shared path and timestamp utilities
├── config/              # Configuration templates and examples
└── examples/            # Systemd service file
```

## Architecture

The backup pipeline follows a clean adapter pattern:

```
Cron Trigger → Dump (Dumper) → Compress (Compressor) → Upload (Uploader) → Notify (Notifier) → Cleanup
```

Each stage is an interface. New backends are added by implementing the interface and registering in the factory function. The scheduler uses dependency injection for all adapter factories, making it fully testable with fakes.

## License

MIT License - see LICENSE file for details
