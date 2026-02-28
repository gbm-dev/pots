#!/bin/bash
# Build D-Modem artifacts once and store them in third_party/dmodem for fast image rebuilds.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${ROOT_DIR}/third_party/dmodem"
DMODEM_REF="${DMODEM_REF:-59cacd766de7e093c9ef2109f146f417f2b6a945}"
DMODEM_MAKE_FLAGS="${DMODEM_MAKE_FLAGS:-NO_PULSE=1}"
DMODEM_PJSIP_CONFIGURE_FLAGS="${DMODEM_PJSIP_CONFIGURE_FLAGS:---enable-epoll --disable-video --disable-sound --enable-ext-sound --disable-speex-aec --enable-g711-codec --disable-l16-codec --disable-gsm-codec --disable-g722-codec --disable-g7221-codec --disable-speex-codec --disable-ilbc-codec --disable-sdl --disable-ffmpeg --disable-v4l2 --disable-openh264 --disable-vpx --disable-android-mediacodec --disable-darwin-ssl --disable-ssl --disable-opencore-amr --disable-silk --disable-opus --disable-bcg729 --disable-libyuv --disable-libwebrtc}"

mkdir -p "${OUT_DIR}"

if ! command -v docker >/dev/null 2>&1; then
    echo "ERROR: docker is required to build dmodem artifacts."
    exit 1
fi

echo "Building D-Modem ref ${DMODEM_REF} (flags: ${DMODEM_MAKE_FLAGS})..."

docker run --rm \
    -v "${OUT_DIR}:/out" \
    ubuntu:24.04 \
    bash -lc "
set -euo pipefail
apt-get update
apt-get install -y --no-install-recommends \
  build-essential gcc-multilib libc6-dev-i386 git pkg-config libssl-dev ca-certificates make
git clone https://git.jerryxiao.cc/Jerry/D-Modem /tmp/dmodem
cd /tmp/dmodem
git checkout '${DMODEM_REF}'
git submodule update --init --recursive
cd /tmp/dmodem/pjproject
./configure --prefix=/tmp/dmodem/pjsip.install ${DMODEM_PJSIP_CONFIGURE_FLAGS}
cd /tmp/dmodem
make ${DMODEM_MAKE_FLAGS}
cp /tmp/dmodem/slmodemd/slmodemd /out/slmodemd
cp /tmp/dmodem/d-modem /out/d-modem
chmod 0755 /out/slmodemd /out/d-modem
"

echo "Artifacts written to ${OUT_DIR}:"
ls -lh "${OUT_DIR}/slmodemd" "${OUT_DIR}/d-modem"
