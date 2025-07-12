FROM golang:1.24.1-alpine

# Install Go build tools, make, migration tools, and other common packages
RUN apk add --no-cache \
    make \
    bash \
    gcc \
    musl-dev \
    tzdata \
    curl \
    git \
    sqlite \
    mysql-client \
    mariadb-connector-c-dev \
    ca-certificates

# Set working directory
WORKDIR /app

# Copy dependency files and download them
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code and Makefile
COPY . .

# Optional: build the Go binary
RUN go build -o bin/main ./cmd/api/*.go

# Install golang-migrate with MySQL support
RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Add Go binaries to PATH
ENV PATH="/go/bin:$PATH"

# Expose app port
EXPOSE 8080

# Default command
CMD ["/app/bin/main"]
