FROM golang:1.25.0-alpine

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

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Create necessary directories
RUN mkdir -p cmd/migrate/migrations

# Build the application
RUN go build -o main ./cmd/api/*.go

# Install migrate tool
RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

RUN chmod +x ./main

EXPOSE 8080

# Add a health check to help debugging
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

CMD ["./main"]