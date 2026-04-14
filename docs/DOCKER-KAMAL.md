# Docker & Kamal Deployment

This guide covers deploying dumptruckd as a [Kamal](https://kamal-deploy.org/) accessory using Docker. dumptruckd runs entirely from environment variables in this mode -- no config files needed.

## How It Works

When no config file is found and no `-config` flag is provided, dumptruckd checks for `DUMPTRUCKD_DB_TYPE`. If set, it builds a single backup job from `DUMPTRUCKD_*` environment variables. This is the recommended mode for Kamal accessories.

## Container-Name-as-Host Pattern

In Docker, containers on the same network can reach each other by container name. Kamal names accessories using the pattern `<service>-<accessory>`, so a database accessory named `db` in a service called `myapp` becomes the container `myapp-db`.

Set `DUMPTRUCKD_DB_HOST` to the database container name:

```yaml
DUMPTRUCKD_DB_HOST: myapp-db
```

This works because Docker's embedded DNS resolves container names to their internal IP addresses -- but only when both containers share a Docker network.

## Shared Docker Network

For hostname resolution to work, the dumptruckd container and the database container must be on the same Docker network.

Use Kamal's top-level `network:` key to create a shared network. This automatically attaches all accessories to it. Do not also set `options: network:` on individual accessories -- that causes a "network specified multiple times" error.

```yaml
# Top-level -- applies to all containers
network: myapp-private

accessories:
  db:
    image: postgres:17-alpine
    # No options: network: needed -- top-level network handles it
    ...

  dumptruckd:
    image: ghcr.io/saadlulu/dumptruckd:latest
    # No options: network: needed -- top-level network handles it
    ...
```

If you need accessories on different networks, use `options: network:` per-accessory instead of the top-level `network:` key. Do not mix both approaches.

## Complete Kamal `deploy.yml` Example

This is a realistic Kamal configuration with a Postgres database and dumptruckd backing it up to S3 every 6 hours:

```yaml
service: myapp

image: myapp

servers:
  web:
    hosts:
      - 1.2.3.4

registry:
  server: ghcr.io
  username: your-github-user
  password:
    - KAMAL_REGISTRY_PASSWORD

# Shared Docker network -- all accessories are attached automatically.
# Do NOT also set options: network: on individual accessories.
network: myapp-private

accessories:
  # -- PostgreSQL Database --
  db:
    image: postgres:17-alpine
    host: 1.2.3.4
    port: "127.0.0.1:5432:5432"
    env:
      clear:
        POSTGRES_DB: myapp_production
        POSTGRES_USER: myapp
      secret:
        - POSTGRES_PASSWORD
    directories:
      - data:/var/lib/postgresql/data

  # -- dumptruckd Backup Daemon --
  dumptruckd:
    image: ghcr.io/saadlulu/dumptruckd:latest
    host: 1.2.3.4
    env:
      clear:
        # Database connection -- uses the db container name as host
        DUMPTRUCKD_DB_TYPE: postgres
        DUMPTRUCKD_DB_HOST: myapp-db
        DUMPTRUCKD_DB_PORT: "5432"
        DUMPTRUCKD_DB_NAME: myapp_production
        DUMPTRUCKD_DB_USER: myapp

        # Upload to S3
        DUMPTRUCKD_UPLOAD_TYPE: s3
        DUMPTRUCKD_S3_BUCKET: myapp-backups
        DUMPTRUCKD_S3_REGION: us-east-1
        DUMPTRUCKD_S3_PREFIX: db

        # Schedule: every 6 hours (default)
        DUMPTRUCKD_SCHEDULE: "0 */6 * * *"

        # Compression
        DUMPTRUCKD_COMPRESS_TYPE: gzip

        # Retention: keep last 30 days and last 10 backups
        DUMPTRUCKD_RETENTION_DAYS: "30"
        DUMPTRUCKD_RETENTION_KEEP_LAST: "10"

        # Health endpoint for monitoring
        DUMPTRUCKD_HEALTH_ENABLED: "true"
        DUMPTRUCKD_HEALTH_PORT: "8080"
      secret:
        # DB_PASSWORD must match POSTGRES_PASSWORD from the db accessory
        - DB_PASSWORD
        - AWS_ACCESS_KEY_ID
        - AWS_SECRET_ACCESS_KEY
```

### Setting Secrets

Kamal reads secrets from `.kamal/secrets`. Add the required values:

```bash
# .kamal/secrets
KAMAL_REGISTRY_PASSWORD=ghp_xxxxxxxxxxxx
POSTGRES_PASSWORD=your-secure-db-password
DB_PASSWORD=your-secure-db-password
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...
```

`DB_PASSWORD` and `POSTGRES_PASSWORD` should be the same value. Postgres uses `POSTGRES_PASSWORD` to set the password; dumptruckd uses `DB_PASSWORD` to connect.

### Deploy

```bash
kamal setup    # First deploy -- creates containers, networks, volumes
kamal deploy   # Subsequent deploys
```

### Verify

Check that dumptruckd is running and can reach the database:

```bash
# View dumptruckd logs
kamal accessory logs dumptruckd

# Run a one-off backup to test
kamal accessory exec dumptruckd --cmd "dumptruckd --once"

# Dry-run to validate config, S3 access, and schedule
kamal accessory exec dumptruckd --cmd "dumptruckd --dry-run"
```

## Local Filesystem Backups

If you prefer local backups instead of S3, mount a volume and set the upload type:

```yaml
  dumptruckd:
    image: ghcr.io/saadlulu/dumptruckd:latest
    host: 1.2.3.4
    env:
      clear:
        DUMPTRUCKD_DB_TYPE: postgres
        DUMPTRUCKD_DB_HOST: myapp-db
        DUMPTRUCKD_DB_PORT: "5432"
        DUMPTRUCKD_DB_NAME: myapp_production
        DUMPTRUCKD_DB_USER: myapp
        DUMPTRUCKD_UPLOAD_TYPE: local
        DUMPTRUCKD_UPLOAD_PATH: /var/backups
        DUMPTRUCKD_RETENTION_DAYS: "30"
        DUMPTRUCKD_RETENTION_KEEP_LAST: "10"
        DUMPTRUCKD_VERIFY: "true"
      secret:
        - DB_PASSWORD
    directories:
      - myapp_backups:/var/backups
```

Backups are written to the mounted volume at `/var/backups/<backup_name>/YYYY/MM/DD/`.

## S3-Compatible Storage (R2, B2, MinIO)

To use an S3-compatible service instead of AWS S3, set the endpoint:

```yaml
env:
  clear:
    DUMPTRUCKD_UPLOAD_TYPE: s3
    DUMPTRUCKD_S3_BUCKET: my-bucket
    DUMPTRUCKD_S3_ENDPOINT: https://your-account.r2.cloudflarestorage.com
    DUMPTRUCKD_S3_REGION: auto
```

When a custom endpoint is set, dumptruckd uses path-style addressing and skips server-side encryption automatically.

## Optional Features via Environment Variables

All features are configurable through environment variables:

```yaml
env:
  clear:
    # Encryption (requires age or gpg in the container)
    DUMPTRUCKD_ENCRYPT_TYPE: age

    # Backup verification after upload
    DUMPTRUCKD_VERIFY: "true"

    # Size anomaly alert threshold (percentage)
    DUMPTRUCKD_SIZE_ALERT_THRESHOLD: "50"

    # Pre/post backup hooks
    DUMPTRUCKD_HOOK_PRE: "/app/scripts/pre-backup.sh"
    DUMPTRUCKD_HOOK_POST: "/app/scripts/post-backup.sh"

    # Notifications
    DUMPTRUCKD_NOTIFY_TYPE: slack
  secret:
    - DUMPTRUCKD_AGE_RECIPIENT
    - SLACK_WEBHOOK_URL
```

## On-Demand Backups

Run a single backup and watch progress in real-time:

```bash
# Foreground -- blocks until done, shows progress logs directly
ssh root@your-server "docker exec your-service-dumptruckd dumptruckd -once"
```

You'll see progress every 5 seconds:

```
level=INFO msg="pg_dump started, this may take several minutes for large databases" database=myapp_production host=myapp-db
level=INFO msg="pg_dump in progress" database=myapp_production size="124.5 MB" elapsed=5s
level=INFO msg="pg_dump in progress" database=myapp_production size="298.1 MB" elapsed=10s
level=INFO msg="pg_dump completed" database=myapp_production size="1.2 GB"
level=INFO msg="backup completed" backup=myapp_production duration=3m42s path=/var/backups/...
```

Do not use `docker exec -d` (detached) -- the progress logs go to the exec process's stdout, not the container's log stream, so `docker logs` won't show them.

From Kamal (blocks until done):

```bash
kamal accessory exec dumptruckd --cmd "dumptruckd --once"
```

Exit code 0 means all backups succeeded; exit code 1 means at least one failed.

## Restore

Restore the latest backup:

```bash
kamal accessory exec dumptruckd --cmd "dumptruckd restore --backup myapp_production --latest"
```

Restore a specific backup by timestamp:

```bash
kamal accessory exec dumptruckd --cmd "dumptruckd restore --backup myapp_production --timestamp 20240115_120000"
```

## Troubleshooting

### "network specified multiple times"

You have both a top-level `network:` key and `options: network:` on an accessory. Use one or the other. If you set `network:` at the top level, remove `options: network:` from all accessories.

### pg_dump version mismatch

```
pg_dump: error: aborting because of server version mismatch
pg_dump: detail: server version: 17.x; pg_dump version: 16.x
```

The dumptruckd Docker image must include a pg_dump version that matches (or is newer than) your PostgreSQL server. As of v0.2.1, the image ships with pg_dump 17 (Alpine 3.21). If you're on an older dumptruckd image, upgrade:

```bash
kamal accessory stop dumptruckd
kamal accessory remove dumptruckd
kamal accessory boot dumptruckd
```

Match your Postgres image version to the dumptruckd pg_dump version. If you use `postgres:17-alpine` for your database, use `dumptruckd:latest` (which includes pg_dump 17). If you use `postgres:16-alpine`, dumptruckd v0.1.0 (pg_dump 16) works, or v0.2.1+ (pg_dump 17 is backward-compatible with PG 16 servers).

### "No config file found" loop

The container starts, prints "No config file found", and exits or loops. This means dumptruckd is looking for a TOML config file instead of reading environment variables.

Cause: older dumptruckd images had `CMD ["-config", "/app/config/dumptruckd.toml"]` as the default, which forces config-file mode even when no file exists. As of v0.2.1, the default CMD is empty so dumptruckd auto-discovers env vars.

If you're on an older image, override CMD in your Kamal config:

```yaml
  dumptruckd:
    image: ghcr.io/saadlulu/dumptruckd:latest
    cmd: dumptruckd
    ...
```

Or upgrade to v0.2.1+.

### "permission denied" writing to /var/backups

```
create destination directory: mkdir /var/backups/myapp_production: permission denied
```

The dumptruckd container runs as uid 1000 but the mounted volume directory was created as root by Kamal/Docker. Fix the ownership on the host:

```bash
ssh root@your-server "chown -R 1000:1000 your-service-dumptruckd/myapp_backups"
```

As of v0.2.2, the Docker image pre-creates `/var/backups/dumptruckd` with correct ownership. If you mount a volume to a different path (e.g., `/var/backups`), you may still need to fix permissions on the host directory after Kamal creates it.

### "connection refused" or DNS errors

The dumptruckd and database containers are not on the same Docker network. Ensure you have a top-level `network:` in your `deploy.yml`, or that both accessories share the same `options: network:` value.

### "DB_PASSWORD not set"

Add `DB_PASSWORD` to the `secret` list in the dumptruckd accessory and to `.kamal/secrets`.

### "no backups configured"

`DUMPTRUCKD_DB_TYPE` is not set. This is the trigger for env-var mode.

### Backups run but upload fails

Check `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` are in the secrets. For S3-compatible services, verify `DUMPTRUCKD_S3_ENDPOINT` is correct.
