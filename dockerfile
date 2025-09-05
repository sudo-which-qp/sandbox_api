FROM golang:1.25-alpine

RUN apk add --no-cache \
    make bash git tzdata ca-certificates \
    sqlite mysql-client

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=0 GOOS=linux
RUN go build -o bin/main ./cmd/api

EXPOSE 8080
ENTRYPOINT ["/app/bin/main"]
