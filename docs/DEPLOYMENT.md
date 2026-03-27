# Production Deployment

## Install

### Option A: Homebrew (macOS/Linux)

```bash
brew tap Saadlulu/tap
brew install dumptruckd
```

### Option B: APT (Debian/Ubuntu)

```bash
curl -fsSL https://saadlulu.github.io/dumptruckd/setup.sh | sudo bash
sudo apt-get install dumptruckd
```

The deb package installs the binary, example configs to `/etc/dumptruckd/`, a systemd service file, and creates the `dumptruckd` system user.

### Option C: Download Release Binary

```bash
wget https://github.com/Saadlulu/dumptruckd/releases/latest/download/dumptruckd_linux_amd64.tar.gz
tar -xzf dumptruckd_linux_amd64.tar.gz
sudo mv dumptruckd /usr/local/bin/
sudo chmod +x /usr/local/bin/dumptruckd
```

### Option D: Interactive Installer

```bash
sudo bash install.sh
```

The installer walks you through database connection, upload destination, schedule, notifications, and sets up systemd.

### Option E: Docker

```bash
docker pull ghcr.io/dumptruckd/dumptruckd:latest
```

### Option F: Build from Source

```bash
git clone https://github.com/Saadlulu/dumptruckd.git
cd dumptruckd
make build
sudo cp bin/dumptruckd /usr/local/bin/
```

## Install Database Tools

```bash
# PostgreSQL
sudo apt-get install -y postgresql-client

# MySQL
sudo apt-get install -y mysql-client
```

## Directory Setup

If you installed via `apt-get`, this is already done. Otherwise:

```bash
sudo mkdir -p /etc/dumptruckd/config.d
sudo mkdir -p /var/backups/dumptruckd
sudo mkdir -p /var/log/dumptruckd
```

## Configuration

Copy and edit config files:

```bash
sudo cp config/dumptruckd.toml.example /etc/dumptruckd/dumptruckd.toml
sudo cp config/config.d/*.toml /etc/dumptruckd/config.d/
```

See [CONFIGURATION.md](CONFIGURATION.md) for details.

## Environment Variables

Create a secure environment file:

```bash
sudo tee /etc/dumptruckd/.env > /dev/null << 'EOF'
DB_PASSWORD=your_database_password
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK
EOF

sudo chmod 600 /etc/dumptruckd/.env
sudo chown root:root /etc/dumptruckd/.env
```

## Create Database Backup User

```sql
-- PostgreSQL
CREATE USER backup_user WITH PASSWORD 'secure_password';
GRANT CONNECT ON DATABASE your_db TO backup_user;
GRANT USAGE ON SCHEMA public TO backup_user;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO backup_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO backup_user;
```

## Test Configuration

```bash
export $(sudo cat /etc/dumptruckd/.env | xargs)
dumptruckd -test -config /etc/dumptruckd/dumptruckd.toml
```

## Systemd Service

If you installed via `apt-get`, the service file and user are already set up. Just enable and start:

```bash
sudo systemctl enable dumptruckd
sudo systemctl start dumptruckd
```

For manual installs, copy the reference service file:

```bash
sudo cp examples/dumptruckd.service /etc/systemd/system/
```

Create the service user:

```bash
sudo useradd -r -s /bin/false dumptruckd
sudo chown -R dumptruckd:dumptruckd /etc/dumptruckd
sudo chown -R dumptruckd:dumptruckd /var/backups/dumptruckd
sudo chown -R dumptruckd:dumptruckd /var/log/dumptruckd
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable dumptruckd
sudo systemctl start dumptruckd
sudo systemctl status dumptruckd
```

View logs:

```bash
sudo journalctl -u dumptruckd -f
```

## Docker Deployment

```bash
docker run -d \
  --name dumptruckd \
  --restart unless-stopped \
  -v /etc/dumptruckd:/app/config \
  -v /var/backups/dumptruckd:/var/backups/dumptruckd \
  --env-file /etc/dumptruckd/.env \
  ghcr.io/dumptruckd/dumptruckd:latest \
  -config /app/config/dumptruckd.toml
```

## S3 Lifecycle Policy

For S3-based retention, configure a lifecycle rule on your bucket:

1. Go to S3 Console → your bucket → Management → Lifecycle rules
2. Create rule: prefix `db-backups/`, expire objects after N days

## Log Rotation

```bash
sudo tee /etc/logrotate.d/dumptruckd > /dev/null << 'EOF'
/var/log/dumptruckd/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 dumptruckd dumptruckd
}
EOF
```

## Verify Backups

```bash
# S3
aws s3 ls s3://your-bucket/db-backups/ --recursive

# Local
ls -lh /var/backups/dumptruckd/
```

## Quick Reference

```bash
sudo systemctl start dumptruckd     # Start
sudo systemctl stop dumptruckd      # Stop
sudo systemctl restart dumptruckd   # Restart
sudo journalctl -u dumptruckd -f    # Logs
dumptruckd -test -config /etc/dumptruckd/dumptruckd.toml  # Test config
```

## Security Checklist

- [ ] Database backup user has minimal permissions (SELECT only)
- [ ] Environment file has restricted permissions (600)
- [ ] Service runs as non-root user
- [ ] S3 bucket has proper IAM policies
- [ ] Logs don't contain sensitive information
- [ ] Firewall rules restrict database access
