#!/bin/bash
set -euo pipefail

# dumptruckd APT repository setup
# Usage: curl -fsSL https://saadlulu.github.io/dumptruckd/setup.sh | sudo bash

REPO_URL="https://saadlulu.github.io/dumptruckd"
KEYRING="/usr/share/keyrings/dumptruckd.gpg"
LIST="/etc/apt/sources.list.d/dumptruckd.list"

if [ "$(id -u)" -ne 0 ]; then
    echo "Error: run with sudo" >&2
    exit 1
fi

echo "Adding dumptruckd APT repository..."

# Add GPG key
curl -fsSL "${REPO_URL}/dumptruckd.gpg.key" | gpg --dearmor -o "$KEYRING"

# Add repository
echo "deb [signed-by=${KEYRING}] ${REPO_URL} stable main" > "$LIST"

# Update
apt-get update -qq

echo "Done. Install with: sudo apt-get install dumptruckd"
