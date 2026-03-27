# Building dumptruckd

## Quick Build

### Option 1: Using Make (requires Go installed)

```bash
# Install dependencies
make deps

# Build binary
make build

# Binary will be in bin/dumptruckd
./bin/dumptruckd -version
```

### Option 2: Using Go directly

```bash
# Download dependencies
go mod download
go mod tidy

# Build
go build -o bin/dumptruckd ./cmd/dumptruckd

# Run
./bin/dumptruckd -version
```

### Option 3: Install to $GOPATH/bin (makes it available system-wide)

```bash
# Install
make install

# Or directly
go install ./cmd/dumptruckd

# Now you can run from anywhere
dumptruckd -version
```

### Option 4: Using Docker (no Go installation needed)

```bash
# Build Docker image
make docker-build

# Run in Docker
make docker-run

# Or manually
docker build -t dumptruckd .
docker run --rm dumptruckd -version
```

## Installing Go

If you don't have Go installed:

### macOS (using Homebrew)
```bash
brew install go
```

### Linux
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install golang-go

# Or download from https://go.dev/dl/
```

### Verify Installation
```bash
go version
# Should show: go version go1.21.x ...
```

## Development Workflow

```bash
# 1. Download dependencies
make deps

# 2. Build
make build

# 3. Test configuration
./bin/dumptruckd -test -config config/dumptruckd.toml

# 4. Run
./bin/dumptruckd -config config/dumptruckd.toml
```

## Troubleshooting

### "command not found: dumptruckd"
- Build the binary first: `make build`
- Or install it: `make install`
- Or use the full path: `./bin/dumptruckd`

### "go: command not found"
- Install Go (see above)
- Or use Docker: `make docker-build`

### Build errors
- Run `make deps` to download dependencies
- Check Go version: `go version` (needs 1.21+)
- Try `go mod tidy` to fix dependency issues

