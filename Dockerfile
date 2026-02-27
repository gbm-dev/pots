FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
ENV IAXMODEM_VERSION=1.3.4

# Install Asterisk and utility packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    asterisk \
    asterisk-modules \
    asterisk-core-sounds-en-gsm \
    openssh-server \
    dialog \
    expect \
    minicom \
    screen \
    psmisc \
    procps \
    passwd \
    ca-certificates \
    # Build deps for iaxmodem
    build-essential \
    wget \
    libtiff-dev \
    && rm -rf /var/lib/apt/lists/*

# Build iaxmodem from source (bundles its own spandsp + libiax2)
# CFLAGS needed for modern gcc (12+) compatibility with old codebase
ENV CFLAGS="-Wno-implicit-function-declaration -Wno-int-conversion -Wno-incompatible-pointer-types"
RUN wget -O /tmp/iaxmodem.tar.gz \
        "https://sourceforge.net/projects/iaxmodem/files/iaxmodem/iaxmodem-${IAXMODEM_VERSION}.tar.gz/download" \
    && cd /tmp && tar xzf iaxmodem.tar.gz \
    && cd iaxmodem-${IAXMODEM_VERSION} \
    && ./build static \
    && cp iaxmodem /usr/local/bin/iaxmodem \
    && chmod +x /usr/local/bin/iaxmodem \
    && cd / && rm -rf /tmp/iaxmodem*
ENV CFLAGS=

# Remove build deps to keep image smaller
RUN apt-get purge -y build-essential wget libtiff-dev \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*

# Create directories
RUN mkdir -p /run/sshd /var/log/oob-sessions /etc/iaxmodem /var/log/iaxmodem

# Copy Asterisk configuration
COPY config/asterisk/pjsip_wizard.conf /etc/asterisk/pjsip_wizard.conf
COPY config/asterisk/extensions.conf /etc/asterisk/extensions.conf
COPY config/asterisk/iax.conf /etc/asterisk/iax.conf
COPY config/asterisk/modules.conf /etc/asterisk/modules.conf

# Copy IAXmodem template
COPY config/iaxmodem/ /etc/iaxmodem-templates/

# Copy site configuration
COPY config/oob-sites.conf /etc/oob-sites.conf

# Copy scripts
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY scripts/setup-iaxmodem.sh /usr/local/bin/setup-iaxmodem.sh
COPY scripts/oob-menu /usr/local/bin/oob-menu
COPY scripts/oob-healthcheck.sh /usr/local/bin/oob-healthcheck.sh

RUN chmod +x /usr/local/bin/entrypoint.sh \
             /usr/local/bin/setup-iaxmodem.sh \
             /usr/local/bin/oob-menu \
             /usr/local/bin/oob-healthcheck.sh

# Expose ports
# 22   - SSH
# 5060 - SIP (UDP)
# 10000-10100 - RTP media
EXPOSE 22 5060/udp 10000-10100/udp

# Docker-level health check (every 30s, 10s timeout, 3 retries before unhealthy)
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD /usr/local/bin/oob-healthcheck.sh || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
