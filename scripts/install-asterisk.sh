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

# --- Remove apt Asterisk if present ---
if dpkg -l asterisk 2>/dev/null | grep -q '^ii'; then
    echo "Removing apt-installed Asterisk..."
    apt-get remove -y asterisk asterisk-modules asterisk-core-sounds-en 2>/dev/null || true
    apt-get autoremove -y 2>/dev/null || true
    echo "  Done."
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

# --- Configure ---
echo "Configuring Asterisk..."
./configure --with-jansson-bundled 2>&1 | tail -5

# --- Build with minimal modules ---
echo "Building Asterisk (this takes a few minutes)..."
make menuselect.makeopts

# Enable only what we need via menuselect CLI
menuselect/menuselect \
    --disable-all \
    --enable res_pjproject \
    --enable res_pjsip \
    --enable res_pjsip_authenticator_digest \
    --enable res_pjsip_outbound_authenticator_digest \
    --enable res_pjsip_endpoint_identifier_ip \
    --enable res_pjsip_endpoint_identifier_user \
    --enable res_pjsip_outbound_registration \
    --enable res_pjsip_session \
    --enable res_pjsip_sdp_rtp \
    --enable res_pjsip_caller_id \
    --enable res_pjsip_nat \
    --enable res_pjsip_rfc3326 \
    --enable res_pjsip_dtmf_info \
    --enable res_pjsip_logger \
    --enable res_pjsip_config_wizard \
    --enable chan_pjsip \
    --enable res_rtp_asterisk \
    --enable res_sorcery_config \
    --enable res_sorcery_memory \
    --enable res_sorcery_astdb \
    --enable res_timing_timerfd \
    --enable codec_ulaw \
    --enable pbx_config \
    --enable app_dial \
    --enable app_playback \
    --enable format_pcm \
    --enable format_gsm \
    menuselect.makeopts

make -j"$(nproc)"

# --- Install ---
echo "Installing Asterisk..."
make install
make config  # init scripts

# --- Verify ---
echo ""
echo "=== Installed ==="
asterisk -V
echo ""
echo "Asterisk ${ASTERISK_VERSION} LTS installed to /usr/sbin/asterisk"
echo "Modules in /usr/lib/asterisk/modules/"
echo ""
echo "To start: ./scripts/local-dev.sh"
