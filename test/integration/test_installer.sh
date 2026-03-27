#!/bin/bash
set -e

echo "========================================="
echo "  Testing interactive installer"
echo "========================================="
echo ""

# Start PostgreSQL
service postgresql start
sleep 2
sudo -u postgres psql -c "CREATE USER dumptest WITH PASSWORD 'testpass123';" 2>/dev/null || true
sudo -u postgres psql -c "CREATE DATABASE myapp OWNER dumptest;" 2>/dev/null || true
sudo -u postgres psql -d myapp -c "
CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT);
INSERT INTO users (name) VALUES ('Alice'), ('Bob');
GRANT ALL ON ALL TABLES IN SCHEMA public TO dumptest;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO dumptest;
"
echo "✅ PostgreSQL ready"

# Run installer with piped input (simulating user choices)
# Choices: PostgreSQL, localhost, 5432, myapp, dumptest, testpass123,
#          local filesystem, /var/backups/dumptruckd, daily 2am, no notifications,
#          30 days retention, no health endpoint, no systemd, no start
echo ""
echo "Running installer with simulated user input..."
echo ""

printf '%s\n' \
    "1" \
    "localhost" \
    "5432" \
    "myapp" \
    "dumptest" \
    "testpass123" \
    "1" \
    "/var/backups/dumptruckd" \
    "2" \
    "1" \
    "30" \
    "n" \
    "n" \
| /install.sh

echo ""
echo "========================================="
echo "  Installer test complete!"
echo "========================================="
echo ""
echo "Generated config:"
cat /etc/dumptruckd/dumptruckd.toml
echo ""
echo "Env file exists: $(test -f /etc/dumptruckd/.env && echo 'yes' || echo 'no')"
echo "Env file permissions: $(stat -c '%a' /etc/dumptruckd/.env 2>/dev/null || echo 'N/A')"
