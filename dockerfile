FROM golang:1.25-alpine

RUN apk add --no-cache \
    make \
    bash \
    gcc \
    musl-dev \
    tzdata \
    curl \
    git \
    sqlite \
    mysql-client \
    mariadb-connector-c-dev \
    ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
ENV PATH="/go/bin:${PATH}"

ENV CGO_ENABLED=1 GOOS=linux GOARCH=arm64
RUN go build -o bin/main ./cmd/api

EXPOSE 8080

ENTRYPOINT ["/app/bin/main"]
