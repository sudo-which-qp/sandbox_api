FROM golang:1.25.0-alpine

# Install Go build tools, make, migration tools, and other common packages
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

RUN go build -o bin/main ./cmd/api/*.go

RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

ENV PATH="/go/bin:$PATH"

COPY .env /app/.env

EXPOSE 8080

CMD ["/app/bin/main"]
