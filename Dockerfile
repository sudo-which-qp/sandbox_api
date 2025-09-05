# Dokploy will automatically pull the ARM64 version on your Pi
FROM golang:1.25.0-alpine

# Install ALL dependencies needed for building AND running
RUN apk add --no-cache \
    make \
    gcc \
    musl-dev \
    git \
    mysql-client \
    mariadb-connector-c-dev \
    ca-certificates \
    tzdata

WORKDIR /app

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire application source code
COPY . .

# Build the application binary (Dokploy expects this)
RUN go build -o main ./cmd/api/*.go

# Install migrate tool for your make commands
RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Make sure the binary is executable
RUN chmod +x ./main

EXPOSE 8080

# Start the application
CMD ["./main"]