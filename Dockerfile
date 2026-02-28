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
    && rm -rf /var/lib/apt/lists/*

# Install prebuilt iaxmodem binary from GitHub release
RUN wget -O /usr/local/bin/iaxmodem \
        "https://github.com/gbm-dev/pots/releases/download/v0.1.0/iaxmodem" \
    && chmod +x /usr/local/bin/iaxmodem

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

# Copy scripts (only startup/infra scripts)
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY scripts/setup-iaxmodem.sh /usr/local/bin/setup-iaxmodem.sh
COPY scripts/oob-healthcheck.sh /usr/local/bin/oob-healthcheck.sh

RUN chmod +x /usr/local/bin/entrypoint.sh \
             /usr/local/bin/setup-iaxmodem.sh \
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
