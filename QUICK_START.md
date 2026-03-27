# Quick Start: How Modular Config Works

## The Flow

```
┌─────────────────────────────────────────────────────────┐
│  dumptruckd -config config/dumptruckd.toml              │
└─────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│  1. Load main config (dumptruckd.toml)                  │
│     - Reads logging settings                             │
│     - Checks for include directives                     │
└─────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│  2. Load component files                                │
│     ├─ config.d/databases.toml    → [database.*]        │
│     ├─ config.d/uploaders.toml    → [uploader.*]        │
│     ├─ config.d/compressors.toml  → [compressor.*]     │
│     └─ config.d/retention.toml    → [retention.*]       │
└─────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│  3. Load backup jobs                                    │
│     └─ config.d/backups.toml      → [[backup]]          │
│        - Each backup references components by name     │
└─────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│  4. Resolve references                                  │
│     database_ref = "prod_postgres"                      │
│       → finds [database.prod_postgres]                  │
│     upload_ref = "prod_s3"                              │
│       → finds [uploader.prod_s3]                        │
└─────────────────────────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│  5. Start scheduler                                     │
│     - Creates cron jobs for each backup                 │
│     - Runs backups according to schedule                │
└─────────────────────────────────────────────────────────┘
```

## Example: What You Define vs. What Happens

### What You Write:

**`config/config.d/databases.toml`**:
```toml
[database.prod_postgres]
type = "postgres"
host = "db.example.com"
database = "production"
```

**`config/config.d/backups.toml`**:
```toml
[[backup]]
name = "my_backup"
schedule = "0 */6 * * *"
database_ref = "prod_postgres"  # ← References the component above
```

### What dumptruckd Does:

1. Loads `[database.prod_postgres]` into memory
2. Sees `database_ref = "prod_postgres"` in backup job
3. Looks up the component: "prod_postgres" → found!
4. Copies the database config into the backup job
5. Backup job now has full database config to use

## Real Example: 3 Backups, 1 Database Component

**Define once** (`config/config.d/databases.toml`):
```toml
[database.prod_postgres]
type = "postgres"
host = "db.example.com"
database = "production"
username = "backup_user"
```

**Use three times** (`config/config.d/backups.toml`):
```toml
# Backup 1: Every 6 hours
[[backup]]
name = "prod_hourly"
schedule = "0 */6 * * *"
database_ref = "prod_postgres"  # ← Same component
upload_ref = "prod_s3"

# Backup 2: Daily
[[backup]]
name = "prod_daily"
schedule = "0 0 0 * * *"
database_ref = "prod_postgres"  # ← Same component
upload_ref = "prod_s3"

# Backup 3: Weekly
[[backup]]
name = "prod_weekly"
schedule = "0 0 0 * * 0"
database_ref = "prod_postgres"  # ← Same component
upload_ref = "archive_s3"
```

**Result:** One database definition, three different backup schedules using it!

## Key Concepts

### 1. Components are Reusable
```toml
# Define once
[uploader.prod_s3]
type = "s3"
  [uploader.prod_s3.s3]
  bucket = "my-bucket"

# Use in multiple backups
[[backup]]
name = "backup1"
upload_ref = "prod_s3"  # ← Uses prod_s3

[[backup]]
name = "backup2"
upload_ref = "prod_s3"  # ← Also uses prod_s3
```

### 2. Components Can Be Overridden
```toml
# In config.d/a-uploaders.toml
[uploader.prod_s3]
type = "s3"
  [uploader.prod_s3.s3]
  bucket = "old-bucket"

# In config.d/z-uploaders.toml (loaded later)
[uploader.prod_s3]
type = "s3"
  [uploader.prod_s3.s3]
  bucket = "new-bucket"  # ← This one wins!
```

### 3. Mix References and Inline Config
```toml
[[backup]]
name = "mixed_backup"
database_ref = "prod_postgres"  # ← Reference

[backup.compress]
type = "zstd"  # ← Inline (not a reference)
```

## Common Patterns

### Pattern 1: Environment-Based Components
```toml
# config.d/databases.toml
[database.prod]
host = "db-prod.example.com"

[database.staging]
host = "db-staging.example.com"

[database.dev]
host = "localhost"
```

### Pattern 2: Shared Uploaders
```toml
# All production backups use same S3 bucket
[[backup]]
name = "db1_prod"
upload_ref = "prod_s3"

[[backup]]
name = "db2_prod"
upload_ref = "prod_s3"  # Same bucket, different prefix
```

### Pattern 3: Compression Profiles
```toml
[compressor.fast]
type = "gzip"  # Quick, reasonable compression

[compressor.max]
type = "gzip"  # Slower, maximum compression

[compressor.none]
type = "none"  # No compression for dev
```

## See Also

- `USAGE.md` - Detailed step-by-step guide
- `config/README.md` - Configuration reference
- `config/example.toml` - Single-file example
- `config/config.d/*.toml` - Modular examples


