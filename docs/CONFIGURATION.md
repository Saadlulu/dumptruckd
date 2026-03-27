# Configuration Guide

dumptruckd uses TOML configuration with two approaches: modular (recommended) or single-file.

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

## Troubleshooting

**"database component 'X' not found"** — Check the component name matches exactly (case-sensitive) and the file containing it is in `config.d/` or listed in `include`.

**"upload.type is required"** — Use either `upload_ref` or inline `[backup.upload]`, not both.

**Component defined but not found** — Verify TOML syntax and that the file is being loaded (check `config.d/` or `include` list).
