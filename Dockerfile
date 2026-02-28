ARG DMODEM_SOURCE=build

FROM scratch AS dmodem-prebuilt
COPY third_party/dmodem/ /tmp/dmodem-prebuilt/

FROM ubuntu:24.04 AS dmodem-build

ENV DEBIAN_FRONTEND=noninteractive
ARG DMODEM_REF=59cacd766de7e093c9ef2109f146f417f2b6a945
ARG DMODEM_MAKE_FLAGS=NO_PULSE=1
ARG DMODEM_PJSIP_CONFIGURE_FLAGS="--enable-epoll --disable-video --disable-sound --enable-ext-sound --disable-speex-aec --enable-g711-codec --disable-l16-codec --disable-gsm-codec --disable-g722-codec --disable-g7221-codec --disable-speex-codec --disable-ilbc-codec --disable-sdl --disable-ffmpeg --disable-v4l2 --disable-openh264 --disable-vpx --disable-android-mediacodec --disable-darwin-ssl --disable-ssl --disable-opencore-amr --disable-silk --disable-opus --disable-bcg729 --disable-libyuv --disable-libwebrtc"

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
    && cd /tmp/dmodem/pjproject \
    && ./configure --prefix=/tmp/dmodem/pjsip.install ${DMODEM_PJSIP_CONFIGURE_FLAGS} \
    && cd /tmp/dmodem \
    && make ${DMODEM_MAKE_FLAGS} \
    && mkdir -p /tmp/dmodem-prebuilt \
    && cp /tmp/dmodem/slmodemd/slmodemd /tmp/dmodem-prebuilt/slmodemd \
    && cp /tmp/dmodem/d-modem /tmp/dmodem-prebuilt/d-modem

FROM dmodem-${DMODEM_SOURCE} AS dmodem-artifacts

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

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    psmisc \
    procps \
    ca-certificates \
    libc6-i386 \
    && rm -rf /var/lib/apt/lists/*

# Install D-Modem binaries built in the previous stage.
COPY --from=dmodem-artifacts /tmp/dmodem-prebuilt/slmodemd /usr/local/bin/slmodemd
COPY --from=dmodem-artifacts /tmp/dmodem-prebuilt/d-modem /usr/local/bin/d-modem

# Create directories
RUN mkdir -p /var/log/oob-sessions /var/log/dmodem

# Copy site configuration
COPY config/oob-sites.conf /etc/oob-sites.conf

# Copy scripts (only startup/infra scripts)
COPY scripts/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY scripts/start-dmodem.sh /usr/local/bin/start-dmodem.sh
COPY scripts/oob-healthcheck.sh /usr/local/bin/oob-healthcheck.sh

RUN chmod +x /usr/local/bin/entrypoint.sh \
             /usr/local/bin/start-dmodem.sh \
             /usr/local/bin/oob-healthcheck.sh

# Install Go binaries built from local source.
COPY --from=go-builder /out/oob-hub /usr/local/bin/oob-hub
COPY --from=go-builder /out/oob-manage /usr/local/bin/oob-manage

# Expose ports
# 2222 - SSH (Go TUI)
# 5060-5070 - SIP (UDP, one listener per modem instance)
# 10000-10100 - RTP media
EXPOSE 2222/tcp 5060-5070/udp 10000-10100/udp

# Docker-level health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD /usr/local/bin/oob-healthcheck.sh || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
