FROM golang:1.25-alpine AS build

RUN apk add --no-cache bash git build-base make ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a static ARM64 binary for Raspberry Pi 5
ENV CGO_ENABLED=0 GOOS=linux GOARCH=arm64
RUN go build -o /out/app ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache make ca-certificates tzdata && \
    adduser -D -H -s /sbin/nologin appuser

WORKDIR /app

COPY --from=build /out/app /app/app

# Non-root for safety
USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/app"]
