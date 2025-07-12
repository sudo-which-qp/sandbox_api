# ---------- Build Stage ----------
FROM golang:1.24.1-alpine AS builder

# Install necessary build tools and make
RUN apk add --no-cache build-base make

# Set working directory
WORKDIR /app

# Copy dependency files first (to leverage cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code
COPY . .

# Build the Go application
RUN go build -o bin/main ./cmd/api/*.go

# Install golang-migrate CLI with MySQL support
RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest


# ---------- Runtime Stage ----------
FROM alpine:latest

# Install runtime dependencies and make (optional but useful for debugging/migrations)
RUN apk --no-cache add ca-certificates tzdata make

# Set working directory
WORKDIR /app

# Copy built Go binary
COPY --from=builder /app/bin/main /app/bin/main

# Copy migrate binary
COPY --from=builder /go/bin/migrate /app/bin/migrate

# Copy migration files and env config
COPY --from=builder /app/cmd/migrate/migrations /app/cmd/migrate/migrations
COPY --from=builder /app/.env /app/.env

# Expose application port
EXPOSE 8080

# Default command
CMD ["/app/bin/main"]
