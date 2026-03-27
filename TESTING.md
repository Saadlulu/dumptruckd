# Testing Your Configuration

dumptruckd includes a comprehensive test mode that validates your entire configuration without running actual backups.

## Quick Start

```bash
# Test your configuration
dumptruckd -test -config config/dumptruckd.toml

# With verbose output
dumptruckd -test -verbose -config config/dumptruckd.toml
```

## What Gets Tested

### 1. Database Components
- **Connection test**: Verifies database connectivity
- **Test dump**: Creates a small schema-only dump (no data)
- **File validation**: Ensures dump file is created and readable

**Example output:**
```
✅ database.prod_postgres: Connection successful, test dump created
✅ database.staging_postgres: Connection successful, test dump created
```

### 2. Compressor Components
- **Compression test**: Creates a test file and compresses it
- **File validation**: Verifies compressed file is created

**Example output:**
```
✅ compressor.fast: Compression test successful
✅ compressor.none: Compression test successful
```

### 3. Uploader Components
- **Upload test**: Uploads a small test file
- **Verification**: Confirms file exists at destination
- **Cleanup**: Deletes the test file

**Example output:**
```
✅ uploader.prod_s3: Upload, download, and delete test successful
✅ uploader.local_backups: Upload, download, and delete test successful
```

### 4. Notifier Components
- **Notification test**: Sends a test message
- **Channel validation**: Verifies notification was sent

**Example output:**
```
✅ notify.slack (backup: postgres_production): Test notification sent successfully
```

### 5. Full Pipeline Tests
For each backup job, tests the complete pipeline:
- Database dump (schema-only for speed)
- Compression
- Upload
- Verification
- Cleanup

**Example output:**
```
✅ backup.postgres_production.database: Database connection successful
✅ backup.postgres_production.compress: Compression test successful
✅ backup.postgres_production.pipeline: Full pipeline test successful (dump -> compress -> upload -> delete)
```

## Test Output

### Successful Test
```
🧪 dumptruckd Configuration Tester
===================================

Running tests...

Test Results:
-------------
✅ database.prod_postgres: Connection successful, test dump created
✅ compressor.fast: Compression test successful
✅ uploader.prod_s3: Upload, download, and delete test successful
✅ backup.postgres_production.database: Database connection successful
✅ backup.postgres_production.compress: Compression test successful
✅ backup.postgres_production.pipeline: Full pipeline test successful

-------------
Summary: 6 passed, 0 failed, 0 skipped

✅ All tests passed! Your configuration is ready to use.
```

### Failed Test
```
Test Results:
-------------
✅ database.prod_postgres: Connection successful, test dump created
❌ uploader.prod_s3: Upload test failed: AWS credentials not found
✅ backup.postgres_production.database: Database connection successful
❌ backup.postgres_production.pipeline: Full pipeline test failed: upload failed

-------------
Summary: 2 passed, 2 failed, 0 skipped

⚠️  Some tests failed. Please check your configuration.
```

## What Test Files Are Created

### Database Test Dumps
- **Location**: System temp directory
- **Format**: `test_dump_{database}_{timestamp}.sql`
- **Content**: Schema-only (no data) - safe for testing
- **Cleanup**: Automatically deleted after test

### Compression Test Files
- **Location**: System temp directory
- **Format**: `dumptruckd-test-*.txt` and compressed versions
- **Content**: Small test data
- **Cleanup**: Automatically deleted after test

### Upload Test Files
- **Location**: Your configured upload destination
- **Format**: `dumptruckd-test-{component}-{timestamp}/...`
- **Content**: Small test file
- **Cleanup**: Automatically deleted after test (if uploader supports delete)

## Troubleshooting

### Database Connection Failed
```
❌ database.prod_postgres: Failed to connect or dump: connection refused
```

**Solutions:**
- Check database host and port
- Verify network connectivity
- Ensure database is running
- Check firewall rules
- Verify credentials in environment variables

### Upload Test Failed
```
❌ uploader.prod_s3: Upload test failed: AWS credentials not found
```

**Solutions:**
- Set `AWS_ACCESS_KEY_ID` environment variable
- Set `AWS_SECRET_ACCESS_KEY` environment variable
- Verify S3 bucket exists and is accessible
- Check IAM permissions

### Compression Test Failed
```
❌ compressor.fast: Compression test failed: compressor not available
```

**Solutions:**
- Verify compression type is supported
- Check system has required tools installed
- Review compressor configuration

### Pipeline Test Failed
```
❌ backup.my_backup.pipeline: Full pipeline test failed: upload failed
```

**Solutions:**
- Check individual component tests first
- Verify all referenced components exist
- Check component configurations
- Review error messages with `-verbose` flag

## Best Practices

1. **Test before deploying**: Always run tests before starting the daemon
2. **Test after changes**: Re-test after modifying configuration
3. **Use verbose mode**: Use `-verbose` when troubleshooting
4. **Check environment**: Ensure all required environment variables are set
5. **Review test files**: Test files are cleaned up, but verify if tests fail

## Test Limitations

- **Schema-only dumps**: Database tests use schema-only dumps (no data) for speed
- **Small test files**: Upload tests use small files, not full backups
- **No scheduling**: Tests don't validate cron schedule syntax
- **No retention**: Retention policies aren't tested (they're handled by S3 lifecycle)

## Integration with CI/CD

You can use the test command in CI/CD pipelines:

```yaml
# Example GitHub Actions
- name: Test dumptruckd configuration
  run: |
    export DB_PASSWORD=${{ secrets.DB_PASSWORD }}
    export AWS_ACCESS_KEY_ID=${{ secrets.AWS_ACCESS_KEY_ID }}
    export AWS_SECRET_ACCESS_KEY=${{ secrets.AWS_SECRET_ACCESS_KEY }}
    dumptruckd -test -config config/dumptruckd.toml
```


