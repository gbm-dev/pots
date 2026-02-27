# Build stage: compile Go binaries
FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /oob-hub ./cmd/oob-hub
RUN CGO_ENABLED=0 GOOS=linux go build -o /oob-manage ./cmd/oob-manage

# Runtime stage
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Install Asterisk and minimal utilities (no openssh, dialog, expect, minicom, screen, passwd)
RUN apt-get update && apt-get install -y --no-install-recommends \
    asterisk \
    asterisk-modules \
    asterisk-core-sounds-en-gsm \
    psmisc \
    procps \
    ca-certificates \
    wget \
    libtiff6 \
    && rm -rf /var/lib/apt/lists/*

# Install prebuilt iaxmodem binary from GitHub release
RUN wget -O /usr/local/bin/iaxmodem \
        "https://github.com/gbm-dev/pots/releases/download/v0.1.0/iaxmodem" \
    && chmod +x /usr/local/bin/iaxmodem

# Copy Go binaries
COPY --from=builder /oob-hub /usr/local/bin/oob-hub
COPY --from=builder /oob-manage /usr/local/bin/oob-manage

# Create directories
RUN mkdir -p /var/log/oob-sessions /etc/iaxmodem /var/log/iaxmodem

# Copy Asterisk configuration
COPY config/asterisk/pjsip.conf /etc/asterisk/pjsip.conf
COPY config/asterisk/pjsip_wizard.conf /etc/asterisk/pjsip_wizard.conf
COPY config/asterisk/extensions.conf /etc/asterisk/extensions.conf
COPY config/asterisk/iax.conf /etc/asterisk/iax.conf
COPY config/asterisk/modules.conf /etc/asterisk/modules.conf

# Copy IAXmodem template
COPY config/iaxmodem/ /etc/iaxmodem-templates/

# Copy site configuration
COPY config/oob-sites.conf /etc/oob-sites.conf

# Copy scripts (only startup/infra scripts, not TUI/user management)
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY scripts/setup-iaxmodem.sh /usr/local/bin/setup-iaxmodem.sh
COPY scripts/oob-healthcheck.sh /usr/local/bin/oob-healthcheck.sh

RUN chmod +x /usr/local/bin/entrypoint.sh \
             /usr/local/bin/setup-iaxmodem.sh \
             /usr/local/bin/oob-healthcheck.sh

# Expose ports
# 2222 - SSH (Go TUI)
# 5060 - SIP (UDP)
# 10000-10100 - RTP media
EXPOSE 2222/tcp 5060/udp 10000-10100/udp

# Docker-level health check (every 30s, 10s timeout, 3 retries before unhealthy)
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD /usr/local/bin/oob-healthcheck.sh || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
