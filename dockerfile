# Build stage
FROM golang:1.24.1-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache gcc musl-dev

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./

# Download Go modules
RUN go mod download

# Copy the source code
COPY . .

# Build the Go application
RUN go build -o bin/main ./cmd/api/*.go

# Install golang-migrate in the builder stage
RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Final stage
FROM alpine:latest

# Add required runtime dependencies including make
RUN apk --no-cache add ca-certificates tzdata make

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/bin/main /app/bin/main

# Copy the migrate binary from the builder stage
COPY --from=builder /go/bin/migrate /app/bin/migrate

# Copy migration files
COPY --from=builder /app/cmd/migrate/migrations /app/cmd/migrate/migrations

# Copy Makefile
COPY --from=builder /app/Makefile /app/Makefile

COPY .env /app/.env

# Expose port
EXPOSE 8080

# Run the application
CMD ["/app/bin/main"]