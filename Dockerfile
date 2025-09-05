# Build stage
FROM golang:1.25.0-alpine AS builder

RUN apk add --no-cache \
    make \
    gcc \
    musl-dev \
    git \
    mysql-client \
    mariadb-connector-c-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o main ./cmd/api/*.go
RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Runtime stage (but keep it simple)
FROM golang:1.25.0-alpine

# Copy everything from builder including Go tools
COPY --from=builder /go/bin/migrate /go/bin/migrate
COPY --from=builder /app/main /app/main
COPY --from=builder /app/.env /app/.env

WORKDIR /app

# Install runtime dependencies only
RUN apk add --no-cache \
    make \
    mysql-client \
    mariadb-connector-c \
    ca-certificates \
    tzdata

RUN chmod +x ./main

EXPOSE 8080

CMD ["./main"]