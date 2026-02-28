#!/bin/bash
# Build D-Modem artifacts once and store them in third_party/dmodem for fast image rebuilds.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${ROOT_DIR}/third_party/dmodem"
DMODEM_REF="${DMODEM_REF:-59cacd766de7e093c9ef2109f146f417f2b6a945}"
DMODEM_MAKE_FLAGS="${DMODEM_MAKE_FLAGS:-NO_PULSE=1}"

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
make ${DMODEM_MAKE_FLAGS}
cp /tmp/dmodem/slmodemd/slmodemd /out/slmodemd
cp /tmp/dmodem/d-modem /out/d-modem
chmod 0755 /out/slmodemd /out/d-modem
"

echo "Artifacts written to ${OUT_DIR}:"
ls -lh "${OUT_DIR}/slmodemd" "${OUT_DIR}/d-modem"
