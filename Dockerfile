FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Install Asterisk and minimal utilities
RUN dpkg --add-architecture i386 && apt-get update && apt-get install -y --no-install-recommends \
    libc6:i386 \
    asterisk \
    asterisk-modules \
    asterisk-core-sounds-en-gsm \
    psmisc \
    procps \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install prebuilt D-Modem binaries from fork release
ARG DMODEM_VERSION=v0.1.2
RUN wget -O /usr/local/bin/slmodemd \
        "https://github.com/gbm-dev/D-Modem/releases/download/${DMODEM_VERSION}/slmodemd" \
    && wget -O /usr/local/bin/d-modem \
        "https://github.com/gbm-dev/D-Modem/releases/download/${DMODEM_VERSION}/d-modem" \
    && chmod +x /usr/local/bin/slmodemd /usr/local/bin/d-modem

# Create directories
RUN mkdir -p /var/log/oob-sessions

# Copy Asterisk configuration
COPY config/asterisk/pjsip.conf /etc/asterisk/pjsip.conf
COPY config/asterisk/pjsip_wizard.conf /etc/asterisk/pjsip_wizard.conf
COPY config/asterisk/extensions.conf /etc/asterisk/extensions.conf
COPY config/asterisk/modules.conf /etc/asterisk/modules.conf

# Copy site configuration
COPY config/oob-sites.conf /etc/oob-sites.conf

# Copy scripts (only startup/infra scripts)
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY scripts/oob-healthcheck.sh /usr/local/bin/oob-healthcheck.sh

RUN chmod +x /usr/local/bin/entrypoint.sh \
             /usr/local/bin/oob-healthcheck.sh

# Install Go binaries from GitHub release — this layer MUST be after COPY
# so that any repo change (scripts, configs) busts the cache above it.
# Pass --build-arg POTS_VERSION=v1.2.0 to pin, or it downloads latest.
ARG POTS_VERSION=latest
RUN set -eux; \
    if [ "$POTS_VERSION" = "latest" ]; then \
        DL_URL="https://github.com/gbm-dev/pots/releases/latest/download"; \
    else \
        DL_URL="https://github.com/gbm-dev/pots/releases/download/${POTS_VERSION}"; \
    fi; \
    wget -O /usr/local/bin/oob-hub "${DL_URL}/oob-hub" \
    && wget -O /usr/local/bin/oob-manage "${DL_URL}/oob-manage" \
    && chmod +x /usr/local/bin/oob-hub /usr/local/bin/oob-manage

# Expose ports
# 2222 - SSH (Go TUI)
# 5060 - SIP (UDP)
# 10000-10100 - RTP media
EXPOSE 2222/tcp 5060/udp 10000-10100/udp

# Docker-level health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD /usr/local/bin/oob-healthcheck.sh || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
