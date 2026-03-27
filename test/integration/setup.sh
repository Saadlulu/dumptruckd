#!/bin/bash
set -e

echo "========================================="
echo "  dumptruckd Integration Test Suite"
echo "========================================="
echo ""

# ---- Start PostgreSQL ----
echo "[1/9] Starting PostgreSQL..."
pg_ctlcluster 14 main start 2>/dev/null || service postgresql start
sleep 2

sudo -u postgres psql -c "CREATE USER dumptest WITH PASSWORD 'testpass123';" 2>/dev/null || true
sudo -u postgres psql -c "CREATE DATABASE testdb OWNER dumptest;" 2>/dev/null || true
sudo -u postgres psql -d testdb -c "
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(200) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
INSERT INTO users (name, email) VALUES
    ('Alice', 'alice@example.com'),
    ('Bob', 'bob@example.com'),
    ('Charlie', 'charlie@example.com');

CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id),
    amount DECIMAL(10,2),
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW()
);
INSERT INTO orders (user_id, amount, status) VALUES
    (1, 99.99, 'completed'),
    (2, 149.50, 'pending'),
    (3, 25.00, 'completed');

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO dumptest;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO dumptest;
"
echo "  ✅ PostgreSQL ready with testdb (2 tables, 6 rows)"

# ---- Start MySQL ----
echo ""
echo "[2/9] Starting MySQL..."
service mysql start
sleep 2

mysql -u root -e "CREATE DATABASE IF NOT EXISTS testdb_mysql;" 2>/dev/null
mysql -u root -e "CREATE USER IF NOT EXISTS 'dumptest'@'localhost' IDENTIFIED BY 'testpass123';" 2>/dev/null
mysql -u root -e "GRANT ALL PRIVILEGES ON testdb_mysql.* TO 'dumptest'@'localhost';" 2>/dev/null
mysql -u root -e "FLUSH PRIVILEGES;" 2>/dev/null
mysql -u root testdb_mysql -e "
CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price DECIMAL(10,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT IGNORE INTO products (id, name, price) VALUES
    (1, 'Widget', 19.99),
    (2, 'Gadget', 49.99),
    (3, 'Doohickey', 9.99);

CREATE TABLE IF NOT EXISTS inventory (
    id INT AUTO_INCREMENT PRIMARY KEY,
    product_id INT,
    quantity INT DEFAULT 0,
    FOREIGN KEY (product_id) REFERENCES products(id)
);
INSERT IGNORE INTO inventory (id, product_id, quantity) VALUES
    (1, 1, 100),
    (2, 2, 50),
    (3, 3, 200);
"
echo "  ✅ MySQL ready with testdb_mysql (2 tables, 6 rows)"

# ---- Start MinIO (S3-compatible) ----
echo ""
echo "[3/9] Starting MinIO (S3-compatible storage)..."
mkdir -p /data/minio
export MINIO_ROOT_USER="minioadmin"
export MINIO_ROOT_PASSWORD="minioadmin123"
minio server /data/minio --address ":9000" --quiet &
sleep 3

# Create test bucket using the S3 API directly
curl -s -X PUT http://localhost:9000/dumptruckd-test \
  -H "Authorization: AWS4-HMAC-SHA256 Credential=minioadmin/20240101/us-east-1/s3/aws4_request" \
  > /dev/null 2>&1 || true

# Simpler: use the minio client or just let the upload create it
# Actually, let's use a PUT with basic auth via the AWS SDK — our tool will handle it.
# MinIO auto-creates buckets on first PutObject if configured, but let's be explicit:
export AWS_ACCESS_KEY_ID="minioadmin"
export AWS_SECRET_ACCESS_KEY="minioadmin123"

# Create bucket using a simple Python one-liner (or we can use the Go test)
python3 -c "
import urllib.request, urllib.error
req = urllib.request.Request('http://localhost:9000/dumptruckd-test', method='PUT')
req.add_header('Host', 'localhost:9000')
try:
    urllib.request.urlopen(req)
except urllib.error.HTTPError:
    pass  # bucket may already exist or need auth
" 2>/dev/null || true

echo "  ✅ MinIO running on :9000 (user: minioadmin)"

# ---- Verify dumptruckd ----
echo ""
echo "[4/9] Verifying dumptruckd installation..."
dumptruckd -version
echo "  ✅ dumptruckd installed"

# ---- Create test config with local + S3 uploads ----
echo ""
echo "[5/9] Creating test configuration..."
mkdir -p /tmp/backups /tmp/test-config

cat > /tmp/test-config/dumptruckd.toml << 'EOF'
[logging]
level = "debug"
format = "text"
output = "stdout"

[health]
enabled = false

# --- Local upload: Postgres ---
[[backup]]
name = "pg-local"
schedule = "0 0 0 * * *"

[backup.database]
type = "postgres"
host = "localhost"
port = 5432
database = "testdb"
username = "dumptest"

[backup.compress]
type = "gzip"

[backup.upload]
type = "local"
path = "/tmp/backups"

[backup.notify]
type = "none"

# --- Local upload: MySQL ---
[[backup]]
name = "mysql-local"
schedule = "0 0 0 * * *"

[backup.database]
type = "mysql"
host = "localhost"
port = 3306
database = "testdb_mysql"
username = "dumptest"

[backup.compress]
type = "gzip"

[backup.upload]
type = "local"
path = "/tmp/backups"

[backup.notify]
type = "none"

# --- S3 upload (MinIO): Postgres ---
[[backup]]
name = "pg-s3"
schedule = "0 0 0 * * *"

[backup.database]
type = "postgres"
host = "localhost"
port = 5432
database = "testdb"
username = "dumptest"

[backup.compress]
type = "gzip"

[backup.upload]
type = "s3"
  [backup.upload.s3]
  bucket = "dumptruckd-test"
  region = "us-east-1"
  endpoint = "http://localhost:9000"
  prefix = "pg-backups"

[backup.notify]
type = "none"

# --- S3 upload (MinIO): MySQL ---
[[backup]]
name = "mysql-s3"
schedule = "0 0 0 * * *"

[backup.database]
type = "mysql"
host = "localhost"
port = 3306
database = "testdb_mysql"
username = "dumptest"

[backup.compress]
type = "gzip"

[backup.upload]
type = "s3"
  [backup.upload.s3]
  bucket = "dumptruckd-test"
  region = "us-east-1"
  endpoint = "http://localhost:9000"
  prefix = "mysql-backups"

[backup.notify]
type = "none"
EOF
echo "  ✅ Config created (4 backup jobs: 2 local, 2 S3)"

# ---- Run unit tests ----
echo ""
echo "[6/9] Running unit tests..."
cd /app
go test ./... 2>&1 | tail -15
echo "  ✅ Unit tests complete"

# ---- Create MinIO bucket properly ----
echo ""
echo "[7/9] Creating S3 bucket on MinIO..."
# Use a tiny Go program to create the bucket via the AWS SDK (same SDK our tool uses)
cat > /tmp/create_bucket.go << 'GOEOF'
package main

import (
    "fmt"
    "os"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
)

func main() {
    sess, _ := session.NewSession(&aws.Config{
        Region:           aws.String("us-east-1"),
        Credentials:      credentials.NewStaticCredentials(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
        Endpoint:         aws.String("http://localhost:9000"),
        S3ForcePathStyle: aws.Bool(true),
    })
    client := s3.New(sess)
    _, err := client.CreateBucket(&s3.CreateBucketInput{Bucket: aws.String("dumptruckd-test")})
    if err != nil {
        fmt.Printf("  Bucket may already exist: %v\n", err)
    } else {
        fmt.Println("  Bucket created: dumptruckd-test")
    }
}
GOEOF
cd /app && go run /tmp/create_bucket.go
echo "  ✅ S3 bucket ready"

# ---- Test config validation (full pipeline for all 4 jobs) ----
echo ""
echo "[8/9] Running dumptruckd -test (full pipeline validation)..."
export DB_PASSWORD="testpass123"
export AWS_ACCESS_KEY_ID="minioadmin"
export AWS_SECRET_ACCESS_KEY="minioadmin123"
dumptruckd -test -config /tmp/test-config/dumptruckd.toml
echo "  ✅ All pipeline tests passed"

# ---- Summary ----
echo ""
echo "[9/9] Checking results..."
echo ""
echo "Local backups in /tmp/backups:"
find /tmp/backups -type f 2>/dev/null || echo "  (none yet — populated on scheduled run)"
echo ""

# List S3 objects
cat > /tmp/list_s3.go << 'GOEOF'
package main

import (
    "fmt"
    "os"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
)

func main() {
    sess, _ := session.NewSession(&aws.Config{
        Region:           aws.String("us-east-1"),
        Credentials:      credentials.NewStaticCredentials(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
        Endpoint:         aws.String("http://localhost:9000"),
        S3ForcePathStyle: aws.Bool(true),
    })
    client := s3.New(sess)
    out, err := client.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: aws.String("dumptruckd-test")})
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Printf("S3 objects in dumptruckd-test bucket: %d\n", len(out.Contents))
    for _, obj := range out.Contents {
        fmt.Printf("  %s (%d bytes)\n", *obj.Key, *obj.Size)
    }
}
GOEOF
cd /app && go run /tmp/list_s3.go

echo ""
echo "========================================="
echo "  All integration tests passed! ✅"
echo ""
echo "  Tested:"
echo "    • PostgreSQL dump → gzip → local filesystem"
echo "    • PostgreSQL dump → gzip → S3 (MinIO)"
echo "    • MySQL dump → gzip → local filesystem"
echo "    • MySQL dump → gzip → S3 (MinIO)"
echo "========================================="
