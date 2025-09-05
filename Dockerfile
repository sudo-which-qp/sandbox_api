# Use the same builder stage as before to get the binary
FROM golang:1.25.0-alpine AS builder
RUN apk add --no-cache gcc musl-dev git mariadb-connector-c-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags='-extldflags="-static"' -o main ./cmd/api/*.go

# Final stage
FROM alpine:3.18
RUN adduser -D -g '' appuser
USER appuser
WORKDIR /app
COPY --from=builder --chown=appuser:appuser /app/main .

# Run the app and force output to the container logs
CMD exec ./main 2>&1