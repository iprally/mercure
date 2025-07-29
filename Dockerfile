# syntax=docker/dockerfile:1
FROM docker.io/golang:1.24 AS builder
WORKDIR /image
COPY . /image
WORKDIR /image/caddy
RUN go mod tidy && go build -o ../mercure mercure/main.go

FROM caddy:2-alpine
COPY --from=builder /image/mercure /usr/bin/caddy
COPY redis-iam.Caddyfile /etc/caddy/Caddyfile