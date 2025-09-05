# ---- build ----
FROM golang:1.25-alpine AS build
RUN apk add --no-cache bash git build-base ca-certificates tzdata
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -o /out/app ./cmd/api

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -H -s /sbin/nologin appuser
WORKDIR /app
COPY --from=build /out/app /app/app
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/app"]
