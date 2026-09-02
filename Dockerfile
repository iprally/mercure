# syntax=docker/dockerfile:1
FROM docker.io/golang:1.26-alpine AS builder
WORKDIR /image
COPY . /image
WORKDIR /image/caddy
RUN go mod tidy && go build -o ../mercure mercure/main.go

FROM caddy:2-alpine
# openssl security floor. 3.5.7-r0 fixed CVE-2026-34182 (CMS AuthEnvelopedData
# forgery, CVSS 9.1) and CVE-2026-31789. 3.5.8-r0 fixes CVE-2026-63073
# (CVSS 9.8) and CVE-2026-75803 (CVSS 9.1), plus
# CVE-2026-14456/14457/18798/54874/63072/63075/63076.
RUN apk upgrade --no-cache && apk add --no-cache "libssl3>=3.5.8-r0" "libcrypto3>=3.5.8-r0"

LABEL org.opencontainers.image.title=Mercure.rocks
LABEL org.opencontainers.image.description="Real-time made easy"
LABEL org.opencontainers.image.url=https://mercure.rocks
LABEL org.opencontainers.image.documentation=https://mercure.rocks/docs/hub/install
LABEL org.opencontainers.image.source=https://github.com/dunglas/mercure
LABEL org.opencontainers.image.licenses=AGPL-3.0-or-later
LABEL org.opencontainers.image.vendor="Kévin Dunglas"

COPY --from=builder /image/mercure /usr/bin/caddy
COPY Caddyfile /etc/caddy/Caddyfile
COPY dev.Caddyfile /etc/caddy/dev.Caddyfile
RUN chmod +x /usr/bin/caddy

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=5 \
	CMD ["wget", "-q", "--spider", "http://127.0.0.1:2019/mercure/health/ready"]
