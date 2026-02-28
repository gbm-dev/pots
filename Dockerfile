FROM ubuntu:24.04 AS dmodem-builder

ENV DEBIAN_FRONTEND=noninteractive
ARG DMODEM_REF=59cacd766de7e093c9ef2109f146f417f2b6a945

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    gcc-multilib \
    libc6-dev-i386 \
    git \
    pkg-config \
    libssl-dev \
    ca-certificates \
    make \
    && rm -rf /var/lib/apt/lists/*

RUN git clone https://git.jerryxiao.cc/Jerry/D-Modem /tmp/dmodem \
    && cd /tmp/dmodem \
    && git checkout "${DMODEM_REF}" \
    && git submodule update --init --recursive \
    && make NO_PULSE=1

FROM golang:1.25 AS go-builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/oob-hub ./cmd/oob-hub \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/oob-manage ./cmd/oob-manage

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Install Asterisk and minimal utilities
RUN apt-get update && apt-get install -y --no-install-recommends \
    asterisk \
    asterisk-modules \
    asterisk-core-sounds-en-gsm \
    psmisc \
    procps \
    ca-certificates \
    wget \
    libtiff6 \
    libc6-i386 \
    && rm -rf /var/lib/apt/lists/*

# Install prebuilt iaxmodem binary from GitHub release
RUN wget -O /usr/local/bin/iaxmodem \
        "https://github.com/gbm-dev/pots/releases/download/v0.1.0/iaxmodem" \
    && chmod +x /usr/local/bin/iaxmodem

# Install D-Modem binaries built in the previous stage.
COPY --from=dmodem-builder /tmp/dmodem/slmodemd/slmodemd /usr/local/bin/slmodemd
COPY --from=dmodem-builder /tmp/dmodem/d-modem.nopulse /usr/local/bin/d-modem.nopulse

# Create directories
RUN mkdir -p /var/log/oob-sessions /etc/iaxmodem /var/log/iaxmodem /var/log/dmodem

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

# Copy scripts (only startup/infra scripts)
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY scripts/setup-iaxmodem.sh /usr/local/bin/setup-iaxmodem.sh
COPY scripts/start-dmodem.sh /usr/local/bin/start-dmodem.sh
COPY scripts/oob-healthcheck.sh /usr/local/bin/oob-healthcheck.sh

RUN chmod +x /usr/local/bin/entrypoint.sh \
             /usr/local/bin/setup-iaxmodem.sh \
             /usr/local/bin/start-dmodem.sh \
             /usr/local/bin/oob-healthcheck.sh

# Install Go binaries built from local source.
COPY --from=go-builder /out/oob-hub /usr/local/bin/oob-hub
COPY --from=go-builder /out/oob-manage /usr/local/bin/oob-manage

# Expose ports
# 2222 - SSH (Go TUI)
# 5060 - SIP (UDP)
# 10000-10100 - RTP media
EXPOSE 2222/tcp 5060/udp 10000-10100/udp

# Docker-level health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD /usr/local/bin/oob-healthcheck.sh || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
