# Use Debian Bookworm base image
FROM golang:1.25.0-bookworm

# Install dependencies using apt (not apk)
RUN apt-get update && apt-get install -y \
    make \
    gcc \
    git \
    mysql-client \
    default-libmysqlclient-dev \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the application
RUN go build -o main ./cmd/api/*.go

# Install migrate tool
RUN go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Make binary executable
RUN chmod +x ./main

EXPOSE 8080

CMD ["./main"]