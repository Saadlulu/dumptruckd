#!/bin/sh
set -e

# Fix ownership of mounted volumes.
# Docker/Kamal named volumes are created as root:root by default. When the
# container runs as the dumptruckd user (UID 1000), it cannot write to them.
# This entrypoint runs as root, fixes ownership, then drops to the app user.
#
# Directories to fix:
#   /var/backups  — default local upload path (DUMPTRUCKD_UPLOAD_PATH)
#   /app/config   — config mount point

UPLOAD_PATH="${DUMPTRUCKD_UPLOAD_PATH:-/var/backups}"

for dir in "$UPLOAD_PATH" /app/config; do
    if [ -d "$dir" ]; then
        # Only chown if not already owned by dumptruckd (avoid slow recursive chown on large volumes)
        owner=$(stat -c '%u' "$dir" 2>/dev/null || stat -f '%u' "$dir" 2>/dev/null)
        if [ "$owner" != "1000" ]; then
            echo "entrypoint: fixing ownership of $dir (was UID $owner, setting to dumptruckd:1000)"
            chown -R dumptruckd:dumptruckd "$dir"
        fi
    fi
done

# Drop privileges and exec the main process
exec su-exec dumptruckd "$@"
