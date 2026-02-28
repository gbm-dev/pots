#!/bin/bash
# Install Asterisk 22 LTS from source
# Replaces the Ubuntu apt package (20.x) with the latest 22 LTS release.
#
# Usage: sudo ./scripts/install-asterisk.sh

set -euo pipefail

ASTERISK_VERSION="22"
DOWNLOAD_URL="http://downloads.asterisk.org/pub/telephony/asterisk/asterisk-${ASTERISK_VERSION}-current.tar.gz"
BUILD_DIR="/usr/local/src"

if [[ $EUID -ne 0 ]]; then
    echo "ERROR: This script must be run as root (sudo)."
    exit 1
fi

echo "=== Asterisk ${ASTERISK_VERSION} LTS Installer ==="
echo ""

# --- Remove apt Asterisk completely ---
if dpkg -l asterisk 2>/dev/null | grep -qE '^ii|^rc'; then
    echo "Removing apt-installed Asterisk..."
    apt-get remove --purge -y 'asterisk*' 2>/dev/null || true
    apt-get autoremove -y 2>/dev/null || true
    echo "  Done."
fi

# Clean up any leftover apt modules
if [[ -d /usr/lib/x86_64-linux-gnu/asterisk ]]; then
    echo "Removing leftover apt Asterisk modules..."
    rm -rf /usr/lib/x86_64-linux-gnu/asterisk
fi

# --- Install build dependencies ---
echo "Installing build dependencies..."
apt-get update -qq
apt-get install -y \
    build-essential \
    wget \
    libncurses5-dev \
    libssl-dev \
    libxml2-dev \
    libsqlite3-dev \
    uuid-dev \
    libjansson-dev \
    libedit-dev

# --- Download Asterisk ---
echo "Downloading Asterisk ${ASTERISK_VERSION} LTS..."
cd "${BUILD_DIR}"
rm -rf asterisk-${ASTERISK_VERSION}*/
wget -q "${DOWNLOAD_URL}" -O "asterisk-${ASTERISK_VERSION}-current.tar.gz"
tar xzf "asterisk-${ASTERISK_VERSION}-current.tar.gz"
rm -f "asterisk-${ASTERISK_VERSION}-current.tar.gz"

# Find extracted directory (e.g., asterisk-22.2.0)
AST_SRC=$(ls -d asterisk-${ASTERISK_VERSION}.*/ 2>/dev/null | head -1)
if [[ -z "${AST_SRC}" ]]; then
    echo "ERROR: Could not find extracted Asterisk source directory"
    exit 1
fi
cd "${AST_SRC}"

# --- Install Asterisk prerequisites ---
echo "Installing Asterisk prerequisites..."
contrib/scripts/install_prereq install

# --- Configure (install to /usr so it replaces the apt paths) ---
echo "Configuring Asterisk..."
./configure --prefix=/usr --with-jansson-bundled 2>&1 | tail -5

# --- Build ---
echo "Building Asterisk (this takes a few minutes)..."
make -j"$(nproc)"

# --- Install ---
echo "Installing Asterisk..."
make install

# --- Verify ---
echo ""
echo "=== Installed ==="
asterisk -V
echo ""
echo "Module directory:"
ls /usr/lib/asterisk/modules/chan_pjsip.so 2>/dev/null && echo "  chan_pjsip.so: OK" || echo "  chan_pjsip.so: MISSING"
ls /usr/lib/asterisk/modules/res_pjsip.so 2>/dev/null && echo "  res_pjsip.so: OK" || echo "  res_pjsip.so: MISSING"
echo ""
echo "To start: ./scripts/local-dev.sh"
