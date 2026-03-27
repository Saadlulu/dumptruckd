# Quick Run Guide

## Option 1: Run in Docker (No Go Installation Needed)

```bash
# Build the Docker image
docker build -t dumptruckd:dev .

# Run version check
docker run --rm dumptruckd:dev -version

# Test configuration
docker run --rm \
  -v $(pwd)/config:/app/config \
  -e DB_PASSWORD="your-password" \
  -e AWS_ACCESS_KEY_ID="your-key" \
  -e AWS_SECRET_ACCESS_KEY="your-secret" \
  dumptruckd:dev -test -config /app/config/dumptruckd.toml

# Run the daemon
docker run --rm \
  -v $(pwd)/config:/app/config \
  -e DB_PASSWORD="your-password" \
  -e AWS_ACCESS_KEY_ID="your-key" \
  -e AWS_SECRET_ACCESS_KEY="your-secret" \
  dumptruckd:dev -config /app/config/dumptruckd.toml
```

## Option 2: Extract Binary from Docker

```bash
# Create a container and copy the binary out
docker create --name dumptruckd-temp dumptruckd:dev
docker cp dumptruckd-temp:/usr/local/bin/dumptruckd ./bin/dumptruckd
docker rm dumptruckd-temp

# Now you can run it directly
./bin/dumptruckd -version
```

## Option 3: Install Go and Build Locally

```bash
# Install Go (macOS)
brew install go

# Or download from https://go.dev/dl/

# Then build
make deps
make build

# Run
./bin/dumptruckd -version
```

## Quick Commands

```bash
# Test your config
dumptruckd -test -config config/dumptruckd.toml

# Run daemon
dumptruckd -config config/dumptruckd.toml

# Show version
dumptruckd -version
```

