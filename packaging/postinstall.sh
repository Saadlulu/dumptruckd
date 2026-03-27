#!/bin/bash
set -e

# Create system user
if ! id -u dumptruckd &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin dumptruckd
fi

# Create directories
mkdir -p /etc/dumptruckd/config.d /var/backups/dumptruckd

# Set ownership
chown -R dumptruckd:dumptruckd /etc/dumptruckd /var/backups/dumptruckd

# Copy example config if no config exists
if [ ! -f /etc/dumptruckd/dumptruckd.toml ]; then
    if [ -f /etc/dumptruckd/dumptruckd.toml.example ]; then
        cp /etc/dumptruckd/dumptruckd.toml.example /etc/dumptruckd/dumptruckd.toml
        chown dumptruckd:dumptruckd /etc/dumptruckd/dumptruckd.toml
    fi
    echo ""
    echo "dumptruckd installed successfully."
    echo ""
    echo "  Config:  /etc/dumptruckd/dumptruckd.toml"
    echo "  Modules: /etc/dumptruckd/config.d/"
    echo ""
    echo "Next steps:"
    echo "  1. Edit /etc/dumptruckd/dumptruckd.toml"
    echo "  2. Edit /etc/dumptruckd/config.d/*.toml"
    echo "  3. Create /etc/dumptruckd/.env with credentials (chmod 600)"
    echo "  4. Test:  dumptruckd -test -config /etc/dumptruckd/dumptruckd.toml"
    echo "  5. Start: sudo systemctl enable --now dumptruckd"
fi

# Reload systemd
if command -v systemctl &>/dev/null; then
    systemctl daemon-reload
fi
