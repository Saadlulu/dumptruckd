# Docker & Kamal Deployment

This guide covers deploying dumptruckd as a [Kamal](https://kamal-deploy.org/) accessory using Docker. dumptruckd runs entirely from environment variables in this mode — no config files needed.

## How It Works

When no config file is found and no `-config` flag is provided, dumptruckd checks for `DUMPTRUCKD_DB_TYPE`. If set, it builds a single backup job from `DUMPTRUCKD_*` environment variables. This is the recommended mode for Kamal accessories.

## Container-Name-as-Host Pattern

In Docker, containers on the same network can reach each other by container name. Kamal names accessories using the pattern `<service>-<accessory>`, so a database accessory named `db` in a service called `myapp` becomes the container `myapp-db`.

Set `DUMPTRUCKD_DB_HOST` to the database container name:

```yaml
DUMPTRUCKD_DB_HOST: myapp-db
```

This works because Docker's embedded DNS resolves container names to their internal IP addresses — but only when both containers share a Docker network.

## Shared Docker Network

For hostname resolution to work, the dumptruckd container and the database container **must be on the same Docker network**. Kamal supports this via the `options` key on accessories.

Without a shared network, dumptruckd cannot resolve the database container name and connections will fail with a DNS error.

The example below creates a network called `myapp-private` and attaches both the database and dumptruckd accessories to it.

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

# Shared Docker network for container-to-container communication
network: myapp-private

accessories:
  # ── PostgreSQL Database ──────────────────────────────────
  db:
    image: postgres:16-alpine
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
    options:
      network: myapp-private

  # ── dumptruckd Backup Daemon ─────────────────────────────
  dumptruckd:
    image: ghcr.io/saadlulu/dumptruckd:latest
    host: 1.2.3.4
    env:
      clear:
        # Database connection — uses the db container name as host
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
    options:
      network: myapp-private
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

> `DB_PASSWORD` and `POSTGRES_PASSWORD` should be the same value. Postgres uses `POSTGRES_PASSWORD` to set the password; dumptruckd uses `DB_PASSWORD` to connect.

### Deploy

```bash
kamal setup    # First deploy — creates containers, networks, volumes
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

Run a single backup without starting the scheduler:

```bash
kamal accessory exec dumptruckd --cmd "dumptruckd --once"
```

This runs all configured backup jobs once and exits. Exit code 0 means all backups succeeded; exit code 1 means at least one failed.

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

**"connection refused" or DNS errors** — The dumptruckd and database containers are not on the same Docker network. Ensure both accessories have `options: network: myapp-private` (or whatever your network name is).

**"DB_PASSWORD not set"** — Add `DB_PASSWORD` to the `secret` list in the dumptruckd accessory and to `.kamal/secrets`.

**"no backups configured"** — `DUMPTRUCKD_DB_TYPE` is not set. This is the trigger for env-var mode.

**Backups run but upload fails** — Check `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` are in the secrets. For S3-compatible services, verify `DUMPTRUCKD_S3_ENDPOINT` is correct.
