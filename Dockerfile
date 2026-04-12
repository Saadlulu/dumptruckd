# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /build

# Copy go mod files
COPY go.mod ./
COPY go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Generate go.sum and tidy dependencies
RUN go mod tidy

# Build binary -- TARGETPLATFORM is set automatically by Docker Buildx
ARG TARGETPLATFORM
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /bin/dumptruckd ./cmd/dumptruckd

# Runtime stage -- Alpine 3.21 for PostgreSQL 17 client support
FROM alpine:3.21

# Install runtime dependencies
# postgresql17-client provides pg_dump 17 (required for PG 17 servers)
# postgresql-client is kept as fallback for PG 16 and earlier
RUN apk add --no-cache \
    postgresql17-client \
    mysql-client \
    ca-certificates \
    tzdata

# Create non-root user
RUN addgroup -g 1000 dumptruckd && \
    adduser -D -u 1000 -G dumptruckd dumptruckd

WORKDIR /app

# Copy binary from builder
COPY --from=builder /bin/dumptruckd /usr/local/bin/dumptruckd

# Copy example config
COPY config/example-single-file.toml /app/config/example.toml

# Create config and backup directories with correct ownership.
# /var/backups is the default DUMPTRUCKD_UPLOAD_PATH for local uploads.
# Pre-creating it avoids permission errors when Docker volumes are mounted
# (Kamal/Docker create host dirs as root, but the container runs as uid 1000).
RUN mkdir -p /app/config /var/backups/dumptruckd && \
    chown -R dumptruckd:dumptruckd /app /var/backups/dumptruckd

USER dumptruckd

# No default CMD args. When run without arguments, dumptruckd auto-discovers
# config: searches standard paths first, then falls back to DUMPTRUCKD_*
# environment variables. This makes env-var-only mode (Kamal, Docker) work
# without overriding CMD.
#
# To use a config file: docker run ... dumptruckd -config /path/to/config.toml
ENTRYPOINT ["dumptruckd"]
