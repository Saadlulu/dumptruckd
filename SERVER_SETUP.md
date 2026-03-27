# Server Setup Guide

Complete guide to set up dumptruckd on a production server.

## Prerequisites

- Linux server (Ubuntu/Debian recommended)
- PostgreSQL database (or other supported database)
- AWS S3 bucket (or local storage)
- Root/sudo access

## Step 1: Install dumptruckd

### Option A: Using Docker (Recommended)

```bash
# Pull or build the image
docker pull dumptruckd/dumptruckd:latest
# OR build from source
docker build -t dumptruckd:latest .

# Create directories
sudo mkdir -p /etc/dumptruckd/config.d
sudo mkdir -p /var/backups/dumptruckd
sudo mkdir -p /var/log/dumptruckd
```

### Option B: Install Binary

```bash
# Download binary from releases (when available)
# OR build from source
wget https://github.com/dumptruckd/dumptruckd/releases/latest/download/dumptruckd_linux_amd64.tar.gz
tar -xzf dumptruckd_linux_amd64.tar.gz
sudo mv dumptruckd /usr/local/bin/
sudo chmod +x /usr/local/bin/dumptruckd

# Create directories
sudo mkdir -p /etc/dumptruckd/config.d
sudo mkdir -p /var/backups/dumptruckd
sudo mkdir -p /var/log/dumptruckd
```

### Option C: Build from Source

```bash
# Install Go
sudo apt-get update
sudo apt-get install -y golang-go postgresql-client

# Clone and build
git clone https://github.com/dumptruckd/dumptruckd.git
cd dumptruckd
make build
sudo cp bin/dumptruckd /usr/local/bin/
```

## Step 2: Install Database Tools

```bash
# For PostgreSQL
sudo apt-get install -y postgresql-client

# For MySQL (when supported)
# sudo apt-get install -y mysql-client

# Verify installation
pg_dump --version
```

## Step 3: Configure dumptruckd

### 3.1 Create Main Config

```bash
sudo cp /path/to/dumptruckd/config/dumptruckd.toml.example /etc/dumptruckd/dumptruckd.toml
sudo nano /etc/dumptruckd/dumptruckd.toml
```

Edit the logging section:
```toml
[logging]
level = "info"
format = "text"
output = "/var/log/dumptruckd/dumptruckd.log"
```

### 3.2 Configure Database Connection

```bash
sudo nano /etc/dumptruckd/config.d/databases.toml
```

Add your database:
```toml
[database.prod_postgres]
type = "postgres"
host = "localhost"  # or your DB host
port = 5432
database = "your_database_name"
username = "backup_user"  # Create a dedicated backup user
# Password from environment variable
```

### 3.3 Create Database Backup User

```bash
# Connect to PostgreSQL
sudo -u postgres psql

# Create backup user
CREATE USER backup_user WITH PASSWORD 'your_secure_password';
GRANT CONNECT ON DATABASE your_database_name TO backup_user;
GRANT USAGE ON SCHEMA public TO backup_user;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO backup_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO backup_user;

# Exit
\q
```

### 3.4 Configure Upload Destination

#### Option A: S3 (Recommended)

```bash
sudo nano /etc/dumptruckd/config.d/uploaders.toml
```

```toml
[uploader.prod_s3]
type = "s3"
  [uploader.prod_s3.s3]
  bucket = "your-backup-bucket"
  region = "us-east-1"
  prefix = "db-backups"
```

#### Option B: Local Storage

```bash
sudo nano /etc/dumptruckd/config.d/uploaders.toml
```

```toml
[uploader.local]
type = "local"
path = "/var/backups/dumptruckd"
```

### 3.5 Configure Compression

```bash
sudo nano /etc/dumptruckd/config.d/compressors.toml
```

```toml
[compressor.fast]
type = "gzip"
```

### 3.6 Configure Retention

```bash
sudo nano /etc/dumptruckd/config.d/retention.toml
```

```toml
[retention.ten_days]
days = 10
```

### 3.7 Create Backup Job

```bash
sudo nano /etc/dumptruckd/config.d/backups.toml
```

```toml
[[backup]]
name = "production_db"
schedule = "0 */6 * * *"  # Every 6 hours

database_ref = "prod_postgres"
compress_ref = "fast"
upload_ref = "prod_s3"  # or "local" for local storage
retention_ref = "ten_days"

[backup.notify]
type = "slack"  # Optional: for notifications
  [backup.notify.slack]
  webhook_url = "https://hooks.slack.com/services/YOUR/WEBHOOK"
```

## Step 4: Set Environment Variables

### 4.1 Create Environment File

```bash
sudo nano /etc/dumptruckd/.env
```

Add your credentials:
```bash
# Database password
DB_PASSWORD_production=your_database_password

# AWS credentials (if using S3)
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key

# Slack webhook (optional)
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK
```

### 4.2 Secure the Environment File

```bash
sudo chmod 600 /etc/dumptruckd/.env
sudo chown root:root /etc/dumptruckd/.env
```

### 4.3 Load Environment Variables

For systemd service, we'll load them in the service file (see Step 6).

For manual testing:
```bash
export $(cat /etc/dumptruckd/.env | xargs)
```

## Step 5: Test Configuration

```bash
# Load environment variables
export $(sudo cat /etc/dumptruckd/.env | xargs)

# Test configuration
dumptruckd -test -config /etc/dumptruckd/dumptruckd.toml

# If using Docker
docker run --rm \
  -v /etc/dumptruckd:/app/config \
  -v /var/backups/dumptruckd:/var/backups/dumptruckd \
  --env-file /etc/dumptruckd/.env \
  dumptruckd:latest \
  -test -config /app/config/dumptruckd.toml
```

Fix any errors before proceeding!

## Step 6: Set Up systemd Service

### 6.1 Create Service File

```bash
sudo nano /etc/systemd/system/dumptruckd.service
```

```ini
[Unit]
Description=dumptruckd - Database Backup Daemon
Documentation=https://github.com/dumptruckd/dumptruckd
After=network.target postgresql.service

[Service]
Type=simple
User=dumptruckd
Group=dumptruckd
WorkingDirectory=/etc/dumptruckd

# Load environment variables
EnvironmentFile=/etc/dumptruckd/.env

# Binary path (adjust if using Docker)
ExecStart=/usr/local/bin/dumptruckd -config /etc/dumptruckd/dumptruckd.toml

# Restart policy
Restart=on-failure
RestartSec=10s

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/backups/dumptruckd /var/log/dumptruckd

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=dumptruckd

[Install]
WantedBy=multi-user.target
```

### 6.2 Create User

```bash
sudo useradd -r -s /bin/false dumptruckd
sudo chown -R dumptruckd:dumptruckd /etc/dumptruckd
sudo chown -R dumptruckd:dumptruckd /var/backups/dumptruckd
sudo chown -R dumptruckd:dumptruckd /var/log/dumptruckd
```

### 6.3 Enable and Start Service

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable service (start on boot)
sudo systemctl enable dumptruckd

# Start service
sudo systemctl start dumptruckd

# Check status
sudo systemctl status dumptruckd

# View logs
sudo journalctl -u dumptruckd -f
```

## Step 7: Verify It's Working

### 7.1 Check Service Status

```bash
sudo systemctl status dumptruckd
```

### 7.2 Check Logs

```bash
# Recent logs
sudo journalctl -u dumptruckd -n 50

# Follow logs
sudo journalctl -u dumptruckd -f

# Or if using file logging
tail -f /var/log/dumptruckd/dumptruckd.log
```

### 7.3 Verify Backups

#### If using S3:
```bash
aws s3 ls s3://your-backup-bucket/db-backups/
```

#### If using local storage:
```bash
ls -lh /var/backups/dumptruckd/
```

### 7.4 Test Manual Backup

```bash
# Trigger a test (if you add a test command)
# Or wait for scheduled backup
```

## Step 8: Set Up S3 Lifecycle Policy (If Using S3)

This automatically deletes old backups:

1. Go to AWS S3 Console
2. Select your bucket
3. Go to Management → Lifecycle rules
4. Create rule:
   - **Name**: Delete old backups
   - **Prefix**: `db-backups/`
   - **Action**: Expire objects
   - **Days**: 10 (or your retention period)

## Step 9: Monitoring & Alerts

### 9.1 Set Up Log Rotation

```bash
sudo nano /etc/logrotate.d/dumptruckd
```

```
/var/log/dumptruckd/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 dumptruckd dumptruckd
}
```

### 9.2 Monitor Backup Success

Check logs regularly or set up monitoring:

```bash
# Check if backups are running
sudo journalctl -u dumptruckd --since "24 hours ago" | grep "completed"

# Check for errors
sudo journalctl -u dumptruckd --since "24 hours ago" | grep -i error
```

## Troubleshooting

### Service Won't Start

```bash
# Check service status
sudo systemctl status dumptruckd

# Check logs
sudo journalctl -u dumptruckd -n 100

# Test config manually
export $(sudo cat /etc/dumptruckd/.env | xargs)
dumptruckd -test -config /etc/dumptruckd/dumptruckd.toml
```

### Database Connection Failed

```bash
# Test database connection manually
PGPASSWORD=your_password psql -h localhost -U backup_user -d your_database

# Check if user has permissions
sudo -u postgres psql -c "\du backup_user"
```

### S3 Upload Failed

```bash
# Test AWS credentials
aws s3 ls s3://your-backup-bucket/

# Check IAM permissions
# User needs: s3:PutObject, s3:GetObject, s3:DeleteObject
```

### Permission Issues

```bash
# Fix ownership
sudo chown -R dumptruckd:dumptruckd /etc/dumptruckd
sudo chown -R dumptruckd:dumptruckd /var/backups/dumptruckd
sudo chown -R dumptruckd:dumptruckd /var/log/dumptruckd
```

## Quick Reference

```bash
# Start service
sudo systemctl start dumptruckd

# Stop service
sudo systemctl stop dumptruckd

# Restart service
sudo systemctl restart dumptruckd

# View logs
sudo journalctl -u dumptruckd -f

# Test config
dumptruckd -test -config /etc/dumptruckd/dumptruckd.toml

# Check backups (S3)
aws s3 ls s3://your-bucket/db-backups/ --recursive

# Check backups (local)
ls -lh /var/backups/dumptruckd/
```

## Security Checklist

- [ ] Database backup user has minimal permissions (SELECT only)
- [ ] Environment file has restricted permissions (600)
- [ ] Service runs as non-root user
- [ ] S3 bucket has proper IAM policies
- [ ] Logs don't contain sensitive information
- [ ] Firewall rules restrict database access
- [ ] Regular security updates applied

