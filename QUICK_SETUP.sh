#!/bin/bash
# Quick setup script for dumptruckd on a server
# Run with: sudo bash QUICK_SETUP.sh

set -e

echo "🚀 dumptruckd Server Setup"
echo "=========================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "❌ Please run as root (use sudo)"
    exit 1
fi

# Step 1: Create directories
echo "📁 Creating directories..."
mkdir -p /etc/dumptruckd/config.d
mkdir -p /var/backups/dumptruckd
mkdir -p /var/log/dumptruckd
echo "✅ Directories created"

# Step 2: Create user
echo "👤 Creating dumptruckd user..."
if ! id "dumptruckd" &>/dev/null; then
    useradd -r -s /bin/false dumptruckd
    echo "✅ User created"
else
    echo "ℹ️  User already exists"
fi

# Step 3: Set permissions
echo "🔒 Setting permissions..."
chown -R dumptruckd:dumptruckd /etc/dumptruckd
chown -R dumptruckd:dumptruckd /var/backups/dumptruckd
chown -R dumptruckd:dumptruckd /var/log/dumptruckd
chmod 755 /etc/dumptruckd
chmod 755 /var/backups/dumptruckd
chmod 755 /var/log/dumptruckd
echo "✅ Permissions set"

# Step 4: Copy config files (if they exist)
if [ -f "config/dumptruckd.toml.example" ]; then
    echo "📝 Copying config template..."
    if [ ! -f "/etc/dumptruckd/dumptruckd.toml" ]; then
        cp config/dumptruckd.toml.example /etc/dumptruckd/dumptruckd.toml
        echo "✅ Config template copied to /etc/dumptruckd/dumptruckd.toml"
        echo "⚠️  Please edit this file with your settings"
    else
        echo "ℹ️  Config file already exists, skipping"
    fi
fi

# Step 5: Copy component files
if [ -d "config/config.d" ]; then
    echo "📝 Copying component templates..."
    cp -r config/config.d/* /etc/dumptruckd/config.d/ 2>/dev/null || true
    echo "✅ Component templates copied"
    echo "⚠️  Please edit files in /etc/dumptruckd/config.d/"
fi

# Step 6: Create environment file template
echo "🔐 Creating environment file template..."
if [ ! -f "/etc/dumptruckd/.env" ]; then
    cat > /etc/dumptruckd/.env << 'EOF'
# Database passwords
DB_PASSWORD_production=CHANGE_ME

# AWS credentials (if using S3)
AWS_ACCESS_KEY_ID=CHANGE_ME
AWS_SECRET_ACCESS_KEY=CHANGE_ME

# Slack webhook (optional)
SLACK_WEBHOOK_URL=CHANGE_ME
EOF
    chmod 600 /etc/dumptruckd/.env
    chown dumptruckd:dumptruckd /etc/dumptruckd/.env
    echo "✅ Environment file created at /etc/dumptruckd/.env"
    echo "⚠️  Please edit this file with your credentials"
else
    echo "ℹ️  Environment file already exists"
fi

# Step 7: Install systemd service
echo "⚙️  Installing systemd service..."
if [ -f "examples/dumptruckd.service" ]; then
    cp examples/dumptruckd.service /etc/systemd/system/dumptruckd.service
    systemctl daemon-reload
    echo "✅ Service installed"
    echo "⚠️  Edit /etc/systemd/system/dumptruckd.service if needed"
    echo ""
    echo "To enable and start:"
    echo "  sudo systemctl enable dumptruckd"
    echo "  sudo systemctl start dumptruckd"
else
    echo "⚠️  Service file not found, skipping"
fi

# Step 8: Check for required tools
echo "🔧 Checking for required tools..."
MISSING_TOOLS=()

if ! command -v pg_dump &> /dev/null; then
    MISSING_TOOLS+=("postgresql-client")
fi

if [ ${#MISSING_TOOLS[@]} -gt 0 ]; then
    echo "⚠️  Missing tools: ${MISSING_TOOLS[*]}"
    echo "   Install with: sudo apt-get install ${MISSING_TOOLS[*]}"
else
    echo "✅ All required tools found"
fi

echo ""
echo "✅ Setup complete!"
echo ""
echo "Next steps:"
echo "1. Edit /etc/dumptruckd/dumptruckd.toml"
echo "2. Edit /etc/dumptruckd/config.d/*.toml files"
echo "3. Edit /etc/dumptruckd/.env with your credentials"
echo "4. Test: dumptruckd -test -config /etc/dumptruckd/dumptruckd.toml"
echo "5. Start: sudo systemctl start dumptruckd"
echo ""
echo "See SERVER_SETUP.md for detailed instructions"

