# STAGE 1: BUILDER
FROM golang:1.25.0-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    gcc \
    musl-dev \
    git \
    mariadb-connector-c-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build the main application binary statically
RUN CGO_ENABLED=1 go build -ldflags='-extldflags="-static"' -o main ./cmd/api/*.go

# STAGE 2: RUNNER - Create a tiny final image
FROM alpine:3.18

# Create a non-root user for security
RUN adduser -D -g '' appuser
USER appuser

WORKDIR /app

# Copy ONLY the compiled binary from the builder stage
COPY --from=builder --chown=appuser:appuser /app/main .

# Expose the port your application listens on
EXPOSE 8080

# Run the app and force all output (including panics) to be shown in the logs
CMD exec ./main 2>&1