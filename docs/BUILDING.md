# Building dumptruckd

## Prerequisites

- Go 1.23 or later
- `make` (optional, for convenience targets)

## Using Make

```bash
make deps    # Download dependencies
make build   # Build binary → bin/dumptruckd
make install # Install to $GOPATH/bin
```

## Using Go Directly

```bash
go mod download
go build -o bin/dumptruckd ./cmd/dumptruckd
./bin/dumptruckd -version
```

## Docker

```bash
# Build image
make docker-build

# Or directly
docker build -t dumptruckd .
docker run --rm dumptruckd -version
```

## Extract Binary from Docker

If you don't have Go installed:

```bash
docker build -t dumptruckd .
docker create --name dt-tmp dumptruckd
docker cp dt-tmp:/usr/local/bin/dumptruckd ./bin/dumptruckd
docker rm dt-tmp
```

## Development Workflow

```bash
make deps           # Download dependencies
make build          # Build
make test           # Run tests
make test-coverage  # Tests with coverage report
make lint           # Run linters (requires golangci-lint)
make fmt            # Format code
make vet            # Run go vet
```

## Release

The project uses [GoReleaser](https://goreleaser.com/). Version info is injected via ldflags:

```bash
make release          # Build release binaries (snapshot)
make release-dry-run  # Dry run
```

GoReleaser produces binaries for Linux, macOS, and Windows (amd64, arm64, arm), plus deb/rpm packages and Docker images.

## Installing Go

### macOS
```bash
brew install go
```

### Linux
```bash
# Ubuntu/Debian
sudo apt-get install golang-go

# Or download from https://go.dev/dl/
```

### Verify
```bash
go version  # needs 1.23+
```
