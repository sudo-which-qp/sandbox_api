# STAGE 1: BUILDER
FROM --platform=$BUILDPLATFORM golang:1.25.0-alpine AS builder

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

# Build for the target platform
RUN CGO_ENABLED=1 go build -ldflags='-extldflags="-static"' -o main ./cmd/api/*.go
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
COPY --from=builder --chown=appuser:appuser /app/main .
COPY --from=builder --chown=appuser:appuser /app/migrate /usr/local/bin/
COPY --chown=appuser:appuser . .
COPY --chown=appuser:appuser .env .

EXPOSE 8080
CMD ["./main"]