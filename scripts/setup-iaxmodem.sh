#!/bin/bash
# Generate IAXmodem instance configs and matching Asterisk IAX peer configs
# Called by entrypoint.sh at container startup

set -euo pipefail

MODEM_COUNT=${MODEM_COUNT:-8}
IAXMODEM_CONF_DIR=/etc/iaxmodem
ASTERISK_IAX_CONF=/etc/asterisk/iax.conf

echo "Setting up ${MODEM_COUNT} IAXmodem instances..."

mkdir -p "$IAXMODEM_CONF_DIR"

# Generate IAXmodem config files
for i in $(seq 0 $((MODEM_COUNT - 1))); do
    iax_port=$((4570 + i))
    cat > "${IAXMODEM_CONF_DIR}/ttyIAX${i}" <<IAXEOF
device          /dev/ttyIAX${i}
owner           root:root
mode            660
port            ${iax_port}
refresh         300
server          127.0.0.1
peername        iaxmodem${i}
secret          iaxmodem${i}secret
cidname         OOB-Modem-${i}
cidnumber       ${i}
codec           ulaw
IAXEOF
    echo "  Created IAXmodem config for ttyIAX${i} (IAX port ${iax_port})"
done

# Append IAX peer definitions to iax.conf
cat >> "$ASTERISK_IAX_CONF" <<'HEADER'

; === Auto-generated IAXmodem peers ===
HEADER

for i in $(seq 0 $((MODEM_COUNT - 1))); do
    iax_port=$((4570 + i))
    cat >> "$ASTERISK_IAX_CONF" <<PEEREOF

[iaxmodem${i}]
type = friend
secret = iaxmodem${i}secret
context = oob-outbound
host = 127.0.0.1
port = ${iax_port}
disallow = all
allow = ulaw
trunk = no
requirecalltoken = no
PEEREOF
done

echo "IAXmodem setup complete: ${MODEM_COUNT} instances configured."
