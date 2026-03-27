# How to Use dumptruckd - Step by Step Guide

## Quick Start: Modular Configuration

### Step 1: Set Up Your Config Structure

```bash
# Copy the example main config
cp config/dumptruckd.toml.example config/dumptruckd.toml

# The config.d/ directory already has example files
# Edit them to match your setup
```

### Step 2: Define Your Database Connections

Edit `config/config.d/databases.toml`:

```toml
[database.prod_postgres]
type = "postgres"
host = "db-prod.example.com"
port = 5432
database = "production"
username = "backup_user"
# Password comes from environment variable: DB_PASSWORD_production
```

**What this does:** Creates a named component `prod_postgres` that you can reuse in multiple backup jobs.

### Step 3: Define Your Upload Destinations

Edit `config/config.d/uploaders.toml`:

```toml
[uploader.prod_s3]
type = "s3"
  [uploader.prod_s3.s3]
  bucket = "my-backup-bucket"
  region = "us-east-1"
  prefix = "db"
```

**What this does:** Creates a named uploader `prod_s3` that you can reuse.

### Step 4: Define Compression Methods

Edit `config/config.d/compressors.toml`:

```toml
[compressor.fast]
type = "gzip"
```

**What this does:** Creates a named compressor `fast` (gzip compression).

### Step 5: Define Retention Policies

Edit `config/config.d/retention.toml`:

```toml
[retention.ten_days]
days = 10
```

**What this does:** Creates a retention policy that keeps backups for 10 days.

### Step 6: Create Backup Jobs

Edit `config/config.d/backups.toml`:

```toml
[[backup]]
name = "postgres_production"
schedule = "0 */6 * * *"  # Every 6 hours

# Reference the components you defined above
database_ref = "prod_postgres"    # Uses database from databases.toml
compress_ref = "fast"              # Uses compressor from compressors.toml
upload_ref = "prod_s3"              # Uses uploader from uploaders.toml
retention_ref = "ten_days"          # Uses retention from retention.toml

[backup.notify]
type = "slack"
  [backup.notify.slack]
  webhook_url = "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
```

**What this does:** Creates a backup job that runs every 6 hours, using all the components you defined.

### Step 7: Set Environment Variables

```bash
export DB_PASSWORD_production="your-db-password"
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/..."  # Optional
```

### Step 8: Run dumptruckd

```bash
dumptruckd -config config/dumptruckd.toml
```

## How It Works: Component Resolution

When dumptruckd starts, here's what happens:

1. **Loads main config** (`config/dumptruckd.toml`)
   - Reads logging settings
   - Checks for `include` directives

2. **Loads included files** (if specified)
   - Processes files in the order listed
   - Loads all components into memory

3. **Auto-loads config.d/** (if directory exists)
   - Scans `config.d/` for all `.toml` files
   - Loads them alphabetically
   - Later files can override earlier ones

4. **Resolves component references**
   - For each backup job, looks up referenced components:
     - `database_ref = "prod_postgres"` → finds `[database.prod_postgres]`
     - `upload_ref = "prod_s3"` → finds `[uploader.prod_s3]`
     - etc.

5. **Validates configuration**
   - Ensures all references exist
   - Checks required fields are present

6. **Starts scheduler**
   - Creates cron jobs for each backup
   - Runs backups according to schedule

## Real-World Example: Multiple Environments

### Scenario: You have 3 databases (prod, staging, dev) and want different backup strategies

**1. Define all databases once** (`config/config.d/databases.toml`):
```toml
[database.prod_postgres]
type = "postgres"
host = "db-prod.example.com"
database = "production"
username = "backup_user"

[database.staging_postgres]
type = "postgres"
host = "db-staging.example.com"
database = "staging"
username = "backup_user"

[database.dev_postgres]
type = "postgres"
host = "localhost"
database = "development"
username = "backup_user"
```

**2. Define upload destinations** (`config/config.d/uploaders.toml`):
```toml
[uploader.prod_s3]
type = "s3"
  [uploader.prod_s3.s3]
  bucket = "backups-prod"
  region = "us-east-1"

[uploader.staging_s3]
type = "s3"
  [uploader.staging_s3.s3]
  bucket = "backups-staging"
  region = "us-east-1"

[uploader.local]
type = "local"
path = "/var/backups/dev"
```

**3. Create backup jobs** (`config/config.d/backups.toml`):
```toml
# Production: Every 6 hours, to S3, keep 30 days
[[backup]]
name = "prod_backup"
schedule = "0 */6 * * *"
database_ref = "prod_postgres"
compress_ref = "fast"
upload_ref = "prod_s3"
retention_ref = "month"

# Staging: Daily, to S3, keep 7 days
[[backup]]
name = "staging_backup"
schedule = "0 0 0 * * *"
database_ref = "staging_postgres"
compress_ref = "fast"
upload_ref = "staging_s3"
retention_ref = "week"

# Dev: Every 12 hours, local only, keep 3 days
[[backup]]
name = "dev_backup"
schedule = "0 */12 * * *"
database_ref = "dev_postgres"
compress_ref = "none"
upload_ref = "local"
retention_ref = "three_days"
```

**Benefits:**
- ✅ Define each database once, use multiple times
- ✅ Easy to add new backup jobs (just reference existing components)
- ✅ Change a component once, affects all backups using it
- ✅ Clear separation of concerns

## Alternative: Single File Approach

If you prefer everything in one file, you can do this:

**`config/dumptruckd.toml`**:
```toml
[logging]
level = "info"

# Define components inline
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

# Use component references
[[backup]]
name = "my_backup"
schedule = "0 */6 * * *"
database_ref = "prod"
compress_ref = "fast"
upload_ref = "s3"
retention_ref = "ten_days"

# Or use inline config (no references)
[[backup]]
name = "another_backup"
schedule = "0 0 * * *"
  [backup.database]
  type = "postgres"
  host = "localhost"
  database = "other_db"
  username = "backup"
  [backup.compress]
  type = "gzip"
  [backup.upload]
  type = "local"
  path = "/backups"
```

## Mixing Approaches

You can mix component references and inline config:

```toml
# Use referenced database
database_ref = "prod_postgres"

# But override compression inline
[backup.compress]
type = "zstd"  # Different from the referenced compressor
```

## Component Override Order

If the same component name appears in multiple files, the **last one loaded wins**:

1. Main config file
2. Files in `include` (in order)
3. Files in `config.d/` (alphabetically)

So `config.d/z-uploaders.toml` would override `config.d/a-uploaders.toml`.

## Troubleshooting

**Error: "database component 'prod_postgres' not found"**
- Check that the component is defined in one of your config files
- Verify the component name matches exactly (case-sensitive)
- Check that the file containing the component is being loaded

**Error: "upload.type is required"**
- Make sure you're using either `upload_ref` OR inline `[backup.upload]` config
- Don't use both at the same time

**Component not found but it's in the file**
- Check file is in `config.d/` or listed in `include`
- Verify TOML syntax is correct
- Check component name matches (no typos)

## Best Practices

1. **Use descriptive names**: `prod_postgres` not `db1`
2. **Group related components**: All prod components together
3. **Document with comments**: Explain what each component is for
4. **Use environment variables**: Never put passwords in config files
5. **Version control**: Keep config files in git (without secrets)
6. **Test changes**: Validate config before deploying


