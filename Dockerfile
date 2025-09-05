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

# Copy ONLY the necessary files
COPY --from=builder --chown=appuser:appuser /app/main .
COPY --from=builder --chown=appuser:appuser /app/migrate /usr/local/bin/
# Copy the Makefile if you need it for migrations, but NOT .env
COPY --chown=appuser:appuser Makefile .

EXPOSE 8080

CMD ["./main"]
CMD ["./main"]