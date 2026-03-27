FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV GOPATH=/root/go
ENV PATH="/usr/local/go/bin:${GOPATH}/bin:${PATH}"

# Install system deps
RUN apt-get update && apt-get install -y \
    curl \
    git \
    postgresql \
    postgresql-client \
    mysql-server \
    mysql-client \
    sudo \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.23
RUN curl -sL https://go.dev/dl/go1.23.8.linux-amd64.tar.gz | tar -xz -C /usr/local

# Install MinIO (S3-compatible object storage)
RUN wget -q https://dl.min.io/server/minio/release/linux-amd64/minio -O /usr/local/bin/minio \
    && chmod +x /usr/local/bin/minio

# Copy the project
WORKDIR /app
COPY . .

# Build
RUN go mod tidy && go build -o /usr/local/bin/dumptruckd ./cmd/dumptruckd

# Setup script
COPY test/integration/setup.sh /setup.sh
RUN chmod +x /setup.sh

CMD ["/setup.sh"]
