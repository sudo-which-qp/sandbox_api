# STAGE 1: BUILDER
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

RUN CGO_ENABLED=1 go build -ldflags='-extldflags="-static"' -o main ./cmd/app/*.go
RUN CGO_ENABLED=1 go build -ldflags='-extldflags="-static"' -tags 'mysql' -o migrate github.com/golang-migrate/migrate/v4/cmd/migrate

# STAGE 2: RUNNER
FROM alpine:3.18

RUN apk add --no-cache \
    make \
    ca-certificates \
    tzdata

RUN adduser -D -g '' appuser
USER appuser

WORKDIR /app

# Copy the compiled binary
COPY --from=builder --chown=appuser:appuser /app/main .
# Copy the migrate tool
COPY --from=builder --chown=appuser:appuser /app/migrate /usr/local/bin/
# Copy the Makefile
COPY --chown=appuser:appuser Makefile .

# Try copying the entire project to ensure migrations are included
# This is a temporary fix to identify the correct path
COPY --chown=appuser:appuser . .

EXPOSE 8080

CMD ["./main"]