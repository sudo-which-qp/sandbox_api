FROM golang:1.25.0-alpine

# Install build and runtime dependencies
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

# Build the application
RUN go build -o main ./cmd/api/*.go

# Install migrate tool
RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Make sure the binary is executable
RUN chmod +x ./main

EXPOSE 8080

CMD ["./main"]