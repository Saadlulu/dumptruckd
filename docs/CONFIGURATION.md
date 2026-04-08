# Configuration Guide

dumptruckd supports three configuration modes:

1. **TOML file (modular)** — Recommended for bare-metal and systemd deployments
2. **TOML file (single-file)** — Everything in one file
3. **Environment variables only** — Recommended for Docker and Kamal deployments (no config file needed)

## Modular Configuration

The modular approach lets you define reusable components in separate files and reference them by name in backup jobs.

### How It Works

```
dumptruckd -config config/dumptruckd.toml
    │
    ├─ 1. Load main config (logging, health, includes)
    ├─ 2. Load component files from config.d/
    │     ├─ databases.toml    → [database.*]
    │     ├─ uploaders.toml    → [uploader.*]
    │     ├─ compressors.toml  → [compressor.*]
    │     └─ retention.toml    → [retention.*]
    ├─ 3. Load backup jobs from config.d/backups.toml
    ├─ 4. Resolve references (database_ref → [database.name])
    └─ 5. Start scheduler
```

### Setup

```bash
cp config/dumptruckd.toml.example config/dumptruckd.toml
# Edit files in config/config.d/
```

### Step 1: Define Database Connections

Edit `config/config.d/databases.toml`:

```toml
[database.prod_postgres]
type = "postgres"
host = "db-prod.example.com"
port = 5432
database = "production"
username = "backup_user"
# Password from env: DB_PASSWORD_production
```

### Step 2: Define Upload Destinations

Edit `config/config.d/uploaders.toml`:

```toml
[uploader.prod_s3]
type = "s3"
  [uploader.prod_s3.s3]
  bucket = "my-backup-bucket"
  region = "us-east-1"
  prefix = "db"

[uploader.local]
type = "local"
path = "/var/backups/dumptruckd"
```

### Step 3: Define Compression

Edit `config/config.d/compressors.toml`:

```toml
[compressor.fast]
type = "gzip"

[compressor.none]
type = "none"
```

### Step 4: Define Retention

Edit `config/config.d/retention.toml`:

```toml
[retention.ten_days]
days = 10

[retention.month]
days = 30
```

### Step 5: Create Backup Jobs

Edit `config/config.d/backups.toml`:

```toml
[[backup]]
name = "postgres_production"
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

### Step 6: Set Environment Variables

```bash
export DB_PASSWORD_production="your-db-password"
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
```

### Step 7: Run

```bash
dumptruckd -config config/dumptruckd.toml
```

## Single-File Configuration

Everything in one file. See `config/example-single-file.toml` for a complete example.

```bash
cp config/example-single-file.toml config/dumptruckd.toml
```

You can use component references or inline config:

```toml
[logging]
level = "info"

[database.prod]
type = "postgres"
host = "db.example.com"
database = "production"
username = "backup_user"

[uploader.s3]
type = "s3"
  [uploader.s3.s3]
  bucket = "my-bucket"
  region = "us-east-1"

[compressor.fast]
type = "gzip"

[retention.ten_days]
days = 10

[[backup]]
name = "my_backup"
schedule = "0 */6 * * *"
database_ref = "prod"
compress_ref = "fast"
upload_ref = "s3"
retention_ref = "ten_days"
```

## Component Reuse

Define a component once, use it in multiple backup jobs:

```toml
[database.prod_postgres]
type = "postgres"
host = "db.example.com"
database = "production"
username = "backup_user"

[[backup]]
name = "hourly"
schedule = "0 */6 * * *"
database_ref = "prod_postgres"
upload_ref = "prod_s3"

[[backup]]
name = "daily"
schedule = "0 0 * * *"
database_ref = "prod_postgres"
upload_ref = "prod_s3"

[[backup]]
name = "weekly"
schedule = "0 0 * * 0"
database_ref = "prod_postgres"
upload_ref = "archive_s3"
```

## Component Override Order

If the same component name appears in multiple files, the last one loaded wins:

1. Main config file
2. Files in `include` (in order)
3. Files in `config.d/` (alphabetically)

## Logging

```toml
[logging]
level = "info"       # debug, info, warn, error
format = "text"      # text, json
output = "stdout"    # stdout, stderr, or a file path
```

## Health & Metrics

```toml
[health]
enabled = true
port = 8080
```

Set `HEALTH_BEARER_TOKEN` env var to require authentication on health endpoints.

## Config File Lookup

If `-config` is not specified, dumptruckd looks in:

1. `/etc/dumptruckd/dumptruckd.toml`
2. `config/dumptruckd.toml`
3. `dumptruckd.toml`
4. Environment variables (`DUMPTRUCKD_*`) — if no config file is found

When a config file exists (via flag or search), environment variables are ignored entirely.

## Environment Variable Configuration

When no config file is found, dumptruckd can be configured entirely through `DUMPTRUCKD_*` environment variables. This mode is activated when `DUMPTRUCKD_DB_TYPE` is set. It constructs a single backup job.

This is the recommended mode for Docker and Kamal deployments. See [DOCKER-KAMAL.md](DOCKER-KAMAL.md) for a complete Kamal example.

### Environment Variable Reference

| Variable | Config Equivalent | Default | Required |
|---|---|---|---|
| `DUMPTRUCKD_DB_TYPE` | `database.type` | — | Yes |
| `DUMPTRUCKD_DB_HOST` | `database.host` | — | Yes |
| `DUMPTRUCKD_DB_PORT` | `database.port` | 5432 (postgres) / 3306 (mysql) | No |
| `DUMPTRUCKD_DB_NAME` | `database.database` | — | Yes |
| `DUMPTRUCKD_DB_USER` | `database.username` | — | Yes |
| `DUMPTRUCKD_UPLOAD_TYPE` | `upload.type` | — | Yes |
| `DUMPTRUCKD_S3_BUCKET` | `upload.s3.bucket` | — | When upload type is `s3` |
| `DUMPTRUCKD_S3_REGION` | `upload.s3.region` | `us-east-1` | No |
| `DUMPTRUCKD_S3_PREFIX` | `upload.s3.prefix` | — | No |
| `DUMPTRUCKD_S3_ENDPOINT` | `upload.s3.endpoint` | — | No (set for S3-compatible services) |
| `DUMPTRUCKD_UPLOAD_PATH` | `upload.path` | `/var/backups/dumptruckd` | No |
| `DUMPTRUCKD_COMPRESS_TYPE` | `compress.type` | `gzip` | No |
| `DUMPTRUCKD_SCHEDULE` | `schedule` | `0 */6 * * *` | No |
| `DUMPTRUCKD_BACKUP_NAME` | `name` | Value of `DUMPTRUCKD_DB_NAME` | No |
| `DUMPTRUCKD_NOTIFY_TYPE` | `notify.type` | — | No |
| `DUMPTRUCKD_RETENTION_DAYS` | `retention.days` | — | No |
| `DUMPTRUCKD_RETENTION_KEEP_LAST` | `retention.keep_last` | — | No |
| `DUMPTRUCKD_ENCRYPT_TYPE` | `encrypt.type` | — | No |
| `DUMPTRUCKD_VERIFY` | `verify` | `false` | No |
| `DUMPTRUCKD_SIZE_ALERT_THRESHOLD` | `size_alert_threshold` | `50` | No |
| `DUMPTRUCKD_HOOK_PRE` | `hooks.pre` | — | No |
| `DUMPTRUCKD_HOOK_POST` | `hooks.post` | — | No |
| `DUMPTRUCKD_LOG_LEVEL` | `logging.level` | `info` | No |
| `DUMPTRUCKD_LOG_FORMAT` | `logging.format` | `text` | No |
| `DUMPTRUCKD_HEALTH_ENABLED` | `health.enabled` | `false` | No |
| `DUMPTRUCKD_HEALTH_PORT` | `health.port` | `8080` | No |

### Credential Environment Variables

These are used in both TOML and env-var modes:

| Variable | Used By | Notes |
|---|---|---|
| `DB_PASSWORD` | Database dump/restore | Flat form, takes precedence |
| `DB_PASSWORD_{DBNAME}` | Database dump/restore | Suffixed form, fallback |
| `AWS_ACCESS_KEY_ID` | S3 uploader | Required for S3 uploads |
| `AWS_SECRET_ACCESS_KEY` | S3 uploader | Required for S3 uploads |
| `SLACK_WEBHOOK_URL` | Slack notifier | Fallback when not in config |
| `HEALTH_BEARER_TOKEN` | Health endpoint | Optional auth token |
| `DUMPTRUCKD_AGE_RECIPIENT` | Age encryptor | Required when `encrypt.type = age` |
| `DUMPTRUCKD_GPG_RECIPIENT` | GPG encryptor | Required when `encrypt.type = gpg` |

### Hook Environment Variables

These are set automatically when hook commands run:

| Variable | Description |
|---|---|
| `DUMPTRUCKD_HOOK_BACKUP_NAME` | Name of the backup job |
| `DUMPTRUCKD_HOOK_STATUS` | `success` or `failure` |
| `DUMPTRUCKD_HOOK_FILE_PATH` | Path to the backup file |

## Encryption

Encrypt backups before upload using `age` or `gpg`. The encrypted file replaces the unencrypted one, and the appropriate extension (`.age` or `.gpg`) is appended to the filename.

### TOML

```toml
[[backup]]
name = "encrypted_backup"
schedule = "0 */6 * * *"

[backup.encrypt]
type = "age"    # age, gpg, or none
```

Set the recipient via environment variable:

```bash
# For age encryption
export DUMPTRUCKD_AGE_RECIPIENT="age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# For GPG encryption
export DUMPTRUCKD_GPG_RECIPIENT="backup@example.com"
```

### Environment Variables

```bash
export DUMPTRUCKD_ENCRYPT_TYPE=age
export DUMPTRUCKD_AGE_RECIPIENT="age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
```

The `age` or `gpg` binary must be available in `PATH`.

## Pre/Post Backup Hooks

Run custom commands before and after each backup. Hooks receive context about the backup via environment variables.

### TOML

```toml
[[backup]]
name = "hooked_backup"
schedule = "0 */6 * * *"

[backup.hooks]
pre = "/usr/local/bin/pre-backup.sh"
post = "/usr/local/bin/post-backup.sh"
```

### Environment Variables

```bash
export DUMPTRUCKD_HOOK_PRE="/usr/local/bin/pre-backup.sh"
export DUMPTRUCKD_HOOK_POST="/usr/local/bin/post-backup.sh"
```

### Behavior

- Pre-hook failure (non-zero exit) aborts the backup.
- Post-hook failure (non-zero exit) logs a warning but does not mark the backup as failed.
- Hooks have a 60-second timeout.
- Post-hooks run regardless of whether the backup succeeded or failed.

## Backup Verification

Download and validate the backup after upload to confirm it is not corrupted.

### TOML

```toml
[[backup]]
name = "verified_backup"
schedule = "0 */6 * * *"
verify = true
```

### Environment Variables

```bash
export DUMPTRUCKD_VERIFY=true
```

When enabled, dumptruckd downloads the uploaded file, decompresses it, and runs `pg_restore --list` (for Postgres) to validate integrity. Verification failure logs an error and sends a notification but does not mark the backup as failed (the data was already uploaded).

## Size Anomaly Detection

Track backup sizes and alert when a backup deviates significantly from the rolling average.

### TOML

```toml
[[backup]]
name = "tracked_backup"
schedule = "0 */6 * * *"
size_alert_threshold = 50    # percentage deviation (default: 50)
```

### Environment Variables

```bash
export DUMPTRUCKD_SIZE_ALERT_THRESHOLD=50
```

dumptruckd maintains a rolling window of the last 10 backup sizes per job. When a new backup deviates by more than the threshold from the average, an alert is sent via the configured notifier. Anomaly detection is skipped until at least 3 backups have been recorded.

## Retention by Count

Keep the last N backups in addition to (or instead of) retention by days.

### TOML

```toml
[retention.flexible]
days = 30
keep_last = 10
```

### Environment Variables

```bash
export DUMPTRUCKD_RETENTION_DAYS=30
export DUMPTRUCKD_RETENTION_KEEP_LAST=10
```

When both `days` and `keep_last` are set, a file is kept if it satisfies either condition (union policy). Setting `keep_last` to 0 or omitting it disables count-based retention.

## CLI Flags

### `--once`

Run all configured backup jobs once and exit. No scheduler, watchdog, or health server is started.

```bash
dumptruckd --once
dumptruckd --once -config /etc/dumptruckd/dumptruckd.toml
```

Exit code 0 on success, 1 if any backup fails. Designed for single-shot container execution and cron-triggered backups.

### `--dry-run`

Validate the full pipeline without executing any database dumps:

```bash
dumptruckd --dry-run -config /etc/dumptruckd/dumptruckd.toml
```

Dry-run checks:
- Configuration is valid and all adapters can be created
- S3 bucket is accessible (via `HeadBucket` call)
- Test notification is sent if a notifier is configured
- Next 3 scheduled run times are printed for each backup job

Exit code 0 if all checks pass, 1 if any check fails.

### `restore`

Download a backup and restore it into the database:

```bash
# Restore the latest backup
dumptruckd restore --backup my_backup --latest

# Restore a specific backup by timestamp
dumptruckd restore --backup my_backup --timestamp 20240115_120000

# With explicit config
dumptruckd restore --backup my_backup --latest -config /etc/dumptruckd/dumptruckd.toml
```

The restore command downloads from the configured upload destination, decompresses, decrypts if needed, and pipes into the appropriate database tool (`psql` for Postgres, `mysql` for MySQL). Temporary files are cleaned up after restore completes or fails.

## Troubleshooting

**"database component 'X' not found"** — Check the component name matches exactly (case-sensitive) and the file containing it is in `config.d/` or listed in `include`.

**"upload.type is required"** — Use either `upload_ref` or inline `[backup.upload]`, not both.

**Component defined but not found** — Verify TOML syntax and that the file is being loaded (check `config.d/` or `include` list).
