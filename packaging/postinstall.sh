#!/bin/bash
set -e

# Create system user
if ! id -u dumptruckd &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin dumptruckd
fi

# Create directories
mkdir -p /etc/dumptruckd /var/backups/dumptruckd

# Set ownership
chown -R dumptruckd:dumptruckd /etc/dumptruckd /var/backups/dumptruckd

# Copy example config if no config exists
if [ ! -f /etc/dumptruckd/dumptruckd.toml ]; then
    cp /etc/dumptruckd/dumptruckd.toml.example /etc/dumptruckd/dumptruckd.toml
    echo "Default config installed at /etc/dumptruckd/dumptruckd.toml"
    echo "Edit it and run: dumptruckd -test -config /etc/dumptruckd/dumptruckd.toml"
fi

# Reload systemd
if command -v systemctl &>/dev/null; then
    systemctl daemon-reload
    echo ""
    echo "To start dumptruckd:"
    echo "  sudo systemctl enable dumptruckd"
    echo "  sudo systemctl start dumptruckd"
fi
