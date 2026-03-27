#!/usr/bin/env bash
#
# dumptruckd interactive installer
# Usage: curl -sSL https://raw.githubusercontent.com/dumptruckd/dumptruckd/main/install.sh | bash
#
set -euo pipefail

VERSION="${DUMPTRUCKD_VERSION:-latest}"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/dumptruckd"
DATA_DIR="/var/backups/dumptruckd"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}→${NC} $1"; }
ok()    { echo -e "${GREEN}✅${NC} $1"; }
warn()  { echo -e "${YELLOW}⚠${NC} $1"; }
fail()  { echo -e "${RED}✗${NC} $1"; exit 1; }
ask()   { echo -en "${CYAN}?${NC} $1: "; }

# Read password with star masking
read_password() {
    local password=""
    local char=""
    while IFS= read -r -s -n 1 char; do
        if [[ $char == "" ]]; then
            break
        elif [[ $char == $'\x7f' || $char == $'\b' ]]; then
            if [ ${#password} -gt 0 ]; then
                password="${password%?}"
                echo -en "\b \b"
            fi
        else
            password+="$char"
            echo -en "*"
        fi
    done
    echo ""
    printf -v "$1" '%s' "$password"
}

echo ""
echo "========================================="
echo "  dumptruckd installer"
echo "  Database backup daemon"
echo "========================================="
echo ""

# ---- Check root ----
if [ "$EUID" -ne 0 ]; then
    fail "Please run as root: sudo bash install.sh"
fi

# ---- Detect platform ----
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    armv7l)  ARCH="arm" ;;
    *)       fail "Unsupported architecture: $ARCH" ;;
esac

info "Detected platform: ${OS}/${ARCH}"

# ---- Install binary ----
if command -v dumptruckd &>/dev/null; then
    CURRENT=$(dumptruckd -version 2>&1 | head -1)
    warn "dumptruckd is already installed: $CURRENT"
    ask "Reinstall? [y/N]"
    read -r REINSTALL
    if [[ ! "$REINSTALL" =~ ^[Yy]$ ]]; then
        info "Skipping binary install"
    fi
else
    REINSTALL="y"
fi

if [[ "${REINSTALL:-y}" =~ ^[Yy]$ ]]; then
    # Check if binary already exists and works
    if command -v dumptruckd &>/dev/null; then
        ok "Using existing dumptruckd binary"
    else
        info "Installing dumptruckd to ${INSTALL_DIR}..."

        if [ "$VERSION" = "latest" ]; then
            DOWNLOAD_URL="https://github.com/Saadlulu/dumptruckd/releases/latest/download/dumptruckd_${OS}_${ARCH}.tar.gz"
        else
            DOWNLOAD_URL="https://github.com/Saadlulu/dumptruckd/releases/download/${VERSION}/dumptruckd_${VERSION}_${OS}_${ARCH}.tar.gz"
        fi

        TMP_DIR=$(mktemp -d)
        trap "rm -rf $TMP_DIR" EXIT

        if command -v curl &>/dev/null; then
            curl -sSL "$DOWNLOAD_URL" -o "$TMP_DIR/dumptruckd.tar.gz" 2>/dev/null || true
        elif command -v wget &>/dev/null; then
            wget -q "$DOWNLOAD_URL" -O "$TMP_DIR/dumptruckd.tar.gz" 2>/dev/null || true
        fi

        if [ -f "$TMP_DIR/dumptruckd.tar.gz" ] && [ -s "$TMP_DIR/dumptruckd.tar.gz" ]; then
            tar -xzf "$TMP_DIR/dumptruckd.tar.gz" -C "$TMP_DIR" 2>/dev/null || true
        fi

        if [ -f "$TMP_DIR/dumptruckd" ]; then
            mv "$TMP_DIR/dumptruckd" "$INSTALL_DIR/dumptruckd"
            chmod +x "$INSTALL_DIR/dumptruckd"
            ok "Binary installed"
        else
            warn "Could not download release binary. Trying to build from source..."
            if command -v go &>/dev/null; then
                go install github.com/Saadlulu/dumptruckd/cmd/dumptruckd@latest
                cp "$(go env GOPATH)/bin/dumptruckd" "$INSTALL_DIR/dumptruckd"
                ok "Built from source"
            else
                fail "Go is not installed. Install Go first or download a release binary manually."
            fi
        fi
    fi
fi

dumptruckd -version

# ---- Interactive configuration ----
echo ""
echo "========================================="
echo "  Configuration"
echo "========================================="
echo ""

mkdir -p "$CONFIG_DIR" "$DATA_DIR"

# Database type
echo "Which database do you want to back up?"
echo "  1) PostgreSQL"
echo "  2) MySQL"
ask "Choose [1/2]"
read -r DB_CHOICE

case "$DB_CHOICE" in
    2) DB_TYPE="mysql"; DB_PORT_DEFAULT="3306" ;;
    *) DB_TYPE="postgres"; DB_PORT_DEFAULT="5432" ;;
esac
ok "Database: $DB_TYPE"

# Database connection
echo ""
ask "Database host [localhost]"
read -r DB_HOST
DB_HOST="${DB_HOST:-localhost}"

ask "Database port [${DB_PORT_DEFAULT}]"
read -r DB_PORT
DB_PORT="${DB_PORT:-$DB_PORT_DEFAULT}"

ask "Database name"
read -r DB_NAME
while [ -z "$DB_NAME" ]; do
    warn "Database name is required"
    ask "Database name"
    read -r DB_NAME
done

ask "Database username"
read -r DB_USER
while [ -z "$DB_USER" ]; do
    warn "Username is required"
    ask "Database username"
    read -r DB_USER
done

ask "Database password"
read_password DB_PASS
while [ -z "$DB_PASS" ]; do
    warn "Password is required"
    ask "Database password"
    read_password DB_PASS
done

ok "Database connection configured"

# Upload destination
echo ""
echo "Where should backups be stored?"
echo "  1) Local filesystem"
echo "  2) Amazon S3"
echo "  3) S3-compatible (MinIO, DigitalOcean Spaces, etc.)"
ask "Choose [1/2/3]"
read -r UPLOAD_CHOICE

UPLOAD_TYPE="local"
S3_BUCKET=""
S3_REGION=""
S3_ENDPOINT=""
S3_PREFIX=""
LOCAL_PATH="$DATA_DIR"

case "$UPLOAD_CHOICE" in
    2|3)
        UPLOAD_TYPE="s3"
        ask "S3 bucket name"
        read -r S3_BUCKET
        while [ -z "$S3_BUCKET" ]; do
            warn "Bucket name is required"
            ask "S3 bucket name"
            read -r S3_BUCKET
        done

        ask "S3 region [us-east-1]"
        read -r S3_REGION
        S3_REGION="${S3_REGION:-us-east-1}"

        ask "S3 key prefix (folder) [backups]"
        read -r S3_PREFIX
        S3_PREFIX="${S3_PREFIX:-backups}"

        if [ "$UPLOAD_CHOICE" = "3" ]; then
            ask "S3-compatible endpoint URL (e.g. https://minio.example.com:9000)"
            read -r S3_ENDPOINT
        fi

        ask "AWS Access Key ID"
        read -r AWS_KEY
        ask "AWS Secret Access Key"
        read_password AWS_SECRET

        ok "S3 upload configured"
        ;;
    *)
        ask "Backup directory [${DATA_DIR}]"
        read -r LOCAL_PATH
        LOCAL_PATH="${LOCAL_PATH:-$DATA_DIR}"
        mkdir -p "$LOCAL_PATH"
        ok "Local upload to $LOCAL_PATH"
        ;;
esac

# Schedule
echo ""
echo "How often should backups run?"
echo "  1) Every 6 hours"
echo "  2) Daily at 2am"
echo "  3) Daily at midnight"
echo "  4) Weekly (Sunday 2am)"
echo "  5) Custom cron expression"
ask "Choose [1-5]"
read -r SCHED_CHOICE

case "$SCHED_CHOICE" in
    1) SCHEDULE="0 */6 * * *"; SCHED_DESC="every 6 hours" ;;
    3) SCHEDULE="0 0 * * *"; SCHED_DESC="daily at midnight" ;;
    4) SCHEDULE="0 2 * * 0"; SCHED_DESC="weekly Sunday 2am" ;;
    5)
        ask "Cron expression (e.g. 0 2 * * * for daily at 2am)"
        read -r SCHEDULE
        SCHED_DESC="custom"
        ;;
    *) SCHEDULE="0 2 * * *"; SCHED_DESC="daily at 2am" ;;
esac
ok "Schedule: $SCHED_DESC ($SCHEDULE)"

# Notifications
echo ""
echo "Notifications (optional):"
echo "  1) None"
echo "  2) Slack"
echo "  3) Webhook"
ask "Choose [1/2/3]"
read -r NOTIFY_CHOICE

NOTIFY_TYPE="none"
SLACK_URL=""
WEBHOOK_URL=""

case "$NOTIFY_CHOICE" in
    2)
        NOTIFY_TYPE="slack"
        ask "Slack webhook URL"
        read -r SLACK_URL
        ok "Slack notifications enabled"
        ;;
    3)
        NOTIFY_TYPE="webhook"
        ask "Webhook URL"
        read -r WEBHOOK_URL
        ok "Webhook notifications enabled"
        ;;
    *)
        ok "No notifications"
        ;;
esac

# Retention
echo ""
ask "How many days to keep backups? [30]"
read -r RETENTION_DAYS
RETENTION_DAYS="${RETENTION_DAYS:-30}"
ok "Retention: $RETENTION_DAYS days"

# Health endpoint
echo ""
ask "Enable health check endpoint? [y/N]"
read -r HEALTH_ENABLED
HEALTH_PORT="8080"
if [[ "$HEALTH_ENABLED" =~ ^[Yy]$ ]]; then
    ask "Health check port [8080]"
    read -r HEALTH_PORT
    HEALTH_PORT="${HEALTH_PORT:-8080}"
    HEALTH_ENABLED_TOML="true"
    ok "Health endpoint on :${HEALTH_PORT}"
else
    HEALTH_ENABLED_TOML="false"
fi

# ---- Generate config ----
echo ""
echo "========================================="
echo "  Generating configuration"
echo "========================================="
echo ""

BACKUP_NAME="${DB_NAME}-backup"

cat > "${CONFIG_DIR}/dumptruckd.toml" << TOMLEOF
# Generated by dumptruckd installer

[logging]
level = "info"
format = "text"
output = "stdout"

[health]
enabled = ${HEALTH_ENABLED_TOML}
port = ${HEALTH_PORT}

[[backup]]
name = "${BACKUP_NAME}"
schedule = "${SCHEDULE}"

[backup.database]
type = "${DB_TYPE}"
host = "${DB_HOST}"
port = ${DB_PORT}
database = "${DB_NAME}"
username = "${DB_USER}"

[backup.compress]
type = "gzip"

TOMLEOF

# Upload section
if [ "$UPLOAD_TYPE" = "s3" ]; then
    cat >> "${CONFIG_DIR}/dumptruckd.toml" << TOMLEOF
[backup.upload]
type = "s3"
  [backup.upload.s3]
  bucket = "${S3_BUCKET}"
  region = "${S3_REGION}"
  prefix = "${S3_PREFIX}"
TOMLEOF
    if [ -n "$S3_ENDPOINT" ]; then
        echo "  endpoint = \"${S3_ENDPOINT}\"" >> "${CONFIG_DIR}/dumptruckd.toml"
    fi
else
    cat >> "${CONFIG_DIR}/dumptruckd.toml" << TOMLEOF
[backup.upload]
type = "local"
path = "${LOCAL_PATH}"
TOMLEOF
fi

# Retention
cat >> "${CONFIG_DIR}/dumptruckd.toml" << TOMLEOF

[backup.retention]
days = ${RETENTION_DAYS}
TOMLEOF

# Notify section
cat >> "${CONFIG_DIR}/dumptruckd.toml" << TOMLEOF

[backup.notify]
type = "${NOTIFY_TYPE}"
TOMLEOF

if [ "$NOTIFY_TYPE" = "slack" ]; then
    cat >> "${CONFIG_DIR}/dumptruckd.toml" << TOMLEOF
  [backup.notify.slack]
  # webhook_url loaded from SLACK_WEBHOOK_URL env var in .env
TOMLEOF
elif [ "$NOTIFY_TYPE" = "webhook" ]; then
    cat >> "${CONFIG_DIR}/dumptruckd.toml" << TOMLEOF
  [backup.notify.webhook]
  url = "${WEBHOOK_URL}"
TOMLEOF
fi

ok "Config written to ${CONFIG_DIR}/dumptruckd.toml"

# ---- Write environment file ----
cat > "${CONFIG_DIR}/.env" << ENVEOF
DB_PASSWORD=${DB_PASS}
ENVEOF

if [ "$UPLOAD_TYPE" = "s3" ]; then
    cat >> "${CONFIG_DIR}/.env" << ENVEOF
AWS_ACCESS_KEY_ID=${AWS_KEY}
AWS_SECRET_ACCESS_KEY=${AWS_SECRET}
ENVEOF
fi

if [ "$NOTIFY_TYPE" = "slack" ] && [ -n "$SLACK_URL" ]; then
    cat >> "${CONFIG_DIR}/.env" << ENVEOF
SLACK_WEBHOOK_URL=${SLACK_URL}
ENVEOF
fi

chmod 600 "${CONFIG_DIR}/.env"
ok "Credentials written to ${CONFIG_DIR}/.env (mode 600)"

# ---- Setup systemd service ----
echo ""
ask "Install systemd service? [Y/n]"
read -r INSTALL_SERVICE

if [[ ! "$INSTALL_SERVICE" =~ ^[Nn]$ ]]; then
    if ! command -v systemctl &>/dev/null; then
        warn "systemd not available (container or non-systemd OS)"
        info "You can run dumptruckd manually:"
        echo "  source ${CONFIG_DIR}/.env && dumptruckd -config ${CONFIG_DIR}/dumptruckd.toml"
    else
        # Create service user
        if ! id -u dumptruckd &>/dev/null; then
            useradd --system --no-create-home --shell /usr/sbin/nologin dumptruckd 2>/dev/null || true
        fi

        chown -R dumptruckd:dumptruckd "$CONFIG_DIR" 2>/dev/null || true
        chown -R dumptruckd:dumptruckd "$DATA_DIR" 2>/dev/null || true

        cat > /etc/systemd/system/dumptruckd.service << SVCEOF
[Unit]
Description=dumptruckd - Database Backup Daemon
After=network.target postgresql.service mysql.service

[Service]
Type=simple
User=dumptruckd
Group=dumptruckd
EnvironmentFile=${CONFIG_DIR}/.env
ExecStart=${INSTALL_DIR}/dumptruckd -config ${CONFIG_DIR}/dumptruckd.toml
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
SVCEOF

        systemctl daemon-reload
        ok "Systemd service installed"

        ask "Start dumptruckd now? [Y/n]"
        read -r START_NOW
        if [[ ! "$START_NOW" =~ ^[Nn]$ ]]; then
            systemctl enable dumptruckd
            systemctl start dumptruckd
            ok "dumptruckd is running"
        fi
    fi
else
    info "Skipping systemd setup"
    echo ""
    echo "To run manually:"
    echo "  source ${CONFIG_DIR}/.env && dumptruckd -config ${CONFIG_DIR}/dumptruckd.toml"
fi

# ---- Test configuration ----
echo ""
echo "========================================="
echo "  Testing configuration"
echo "========================================="
echo ""

info "Running dumptruckd -test..."
set +e
(
    set -a
    source "${CONFIG_DIR}/.env"
    set +a
    dumptruckd -test -config "${CONFIG_DIR}/dumptruckd.toml"
)
TEST_EXIT=$?
set -e

echo ""
if [ $TEST_EXIT -eq 0 ]; then
    echo "========================================="
    echo "  Installation complete! ✅"
    echo "========================================="
    echo ""
    echo "  Config:  ${CONFIG_DIR}/dumptruckd.toml"
    echo "  Secrets: ${CONFIG_DIR}/.env"
    echo "  Backups: ${LOCAL_PATH:-s3://${S3_BUCKET}/${S3_PREFIX}}"
    echo "  Schedule: ${SCHED_DESC}"
    echo ""
    echo "  Commands:"
    echo "    systemctl status dumptruckd"
    echo "    journalctl -u dumptruckd -f"
    echo "    dumptruckd -test -config ${CONFIG_DIR}/dumptruckd.toml"
    echo ""
else
    warn "Some tests failed. Check your database connection and credentials."
    echo "  Edit: ${CONFIG_DIR}/dumptruckd.toml"
    echo "  Re-test: source ${CONFIG_DIR}/.env && dumptruckd -test -config ${CONFIG_DIR}/dumptruckd.toml"
fi
