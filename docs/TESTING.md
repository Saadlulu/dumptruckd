# Testing Your Configuration

dumptruckd includes a built-in test mode that validates your entire configuration without running actual backups.

## Usage

```bash
dumptruckd -test -config config/dumptruckd.toml

# With verbose output
dumptruckd -test -verbose -config config/dumptruckd.toml
```

## What Gets Tested

### Database Components
- Connection test and small schema-only dump (no data)
- Verifies dump file is created and readable

### Compressor Components
- Creates a test file and compresses it
- Verifies compressed output exists

### Uploader Components
- Uploads a small test file
- Verifies file exists at destination
- Cleans up the test file

### Notifier Components
- Sends a test notification message

### Full Pipeline
For each backup job, runs the complete pipeline: dump → compress → upload → verify → cleanup.

## Example Output

```
🧪 dumptruckd Configuration Tester
===================================

Running tests...

Test Results:
-------------
✅ database.prod_postgres: Connection successful, test dump created
✅ compressor.fast: Compression test successful
✅ uploader.prod_s3: Upload, download, and delete test successful
✅ backup.prod_backup.database: Database connection successful
✅ backup.prod_backup.compress: Compression test successful
✅ backup.prod_backup.pipeline: Full pipeline test successful

Summary: 6 passed, 0 failed, 0 skipped

✅ All tests passed! Your configuration is ready to use.
```

## Troubleshooting

**Database connection failed** — Check host, port, credentials, network connectivity, and firewall rules.

**Upload test failed** — Verify AWS credentials are set, S3 bucket exists, and IAM permissions include `s3:PutObject`, `s3:GetObject`, `s3:DeleteObject`.

**Pipeline test failed** — Check individual component tests first, then verify all referenced components exist.

## Limitations

- Database tests use schema-only dumps (no data) for speed
- Upload tests use small files, not full backups
- Cron schedule syntax is not validated in test mode
- Retention policies are not exercised

## CI/CD Integration

```yaml
# GitHub Actions example
- name: Test configuration
  run: |
    export DB_PASSWORD=${{ secrets.DB_PASSWORD }}
    export AWS_ACCESS_KEY_ID=${{ secrets.AWS_ACCESS_KEY_ID }}
    export AWS_SECRET_ACCESS_KEY=${{ secrets.AWS_SECRET_ACCESS_KEY }}
    dumptruckd -test -config config/dumptruckd.toml
```
