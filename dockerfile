# Build stage
FROM golang:1.24.1-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./

# Download Go modules
RUN go mod download

# Copy the source code
COPY . .

# Build the Go application
RUN go build -o bin/main ./cmd/api/*.go

# Final stage - Use golang image instead of alpine
FROM golang:1.24.1-alpine

# Add required runtime dependencies
RUN apk --no-cache add ca-certificates tzdata make

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/bin/main /app/bin/main

# Copy migration files and Makefile
COPY --from=builder /app/cmd/migrate/migrations /app/cmd/migrate/migrations
COPY --from=builder /app/Makefile /app/Makefile

# Copy source code (needed for go run commands)
COPY --from=builder /app /app

# Install golang-migrate
RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

COPY .env /app/.env

# Expose port
EXPOSE 8080

# Run the application
CMD ["/app/bin/main"]