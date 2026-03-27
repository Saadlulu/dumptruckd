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

# Build binary
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /bin/dumptruckd ./cmd/dumptruckd

# Runtime stage
FROM alpine:3.20

# Install postgresql-client for pg_dump (and other tools as needed)
RUN apk add --no-cache \
    postgresql-client \
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

# Create config directory
RUN mkdir -p /app/config && chown -R dumptruckd:dumptruckd /app

USER dumptruckd

ENTRYPOINT ["dumptruckd"]
CMD ["-config", "/app/config/dumptruckd.toml"]

