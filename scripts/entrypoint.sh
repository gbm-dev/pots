#!/bin/bash
# OOB Console Hub - Container Entrypoint
# Generates configs, substitutes env vars, starts telephony, then launches Go SSH server

set -euo pipefail

echo "=== OOB Console Hub Starting ==="

# --- Substitute Telnyx credentials into PJSIP config ---
PJSIP_CONF=/etc/asterisk/pjsip_wizard.conf
EXTENSIONS_CONF=/etc/asterisk/extensions.conf

escape_sed_replacement() {
    printf '%s' "$1" | sed -e 's/[\/&|\\]/\\&/g'
}

if [[ -z "${TELNYX_SIP_USER:-}" || -z "${TELNYX_SIP_PASS:-}" ]]; then
    echo "WARNING: TELNYX_SIP_USER or TELNYX_SIP_PASS not set!"
    echo "Asterisk will start but Telnyx trunk will not register."
fi

TELNYX_SIP_USER_ESCAPED=$(escape_sed_replacement "${TELNYX_SIP_USER:-unset}")
TELNYX_SIP_PASS_ESCAPED=$(escape_sed_replacement "${TELNYX_SIP_PASS:-unset}")
TELNYX_SIP_DOMAIN_ESCAPED=$(escape_sed_replacement "${TELNYX_SIP_DOMAIN:-sip.telnyx.com}")
TELNYX_OUTBOUND_CID_ESCAPED=$(escape_sed_replacement "${TELNYX_OUTBOUND_CID:-unset}")
TELNYX_OUTBOUND_NAME_ESCAPED=$(escape_sed_replacement "${TELNYX_OUTBOUND_NAME:-OOB-Console-Hub}")

if [[ -z "${TELNYX_OUTBOUND_CID:-}" ]]; then
    echo "WARNING: TELNYX_OUTBOUND_CID not set!"
    echo "Outbound calls may fail with provider errors like 403 Caller Origination Number is Invalid."
fi

sed -i "s|TELNYX_SIP_USER_PLACEHOLDER|${TELNYX_SIP_USER_ESCAPED}|g" "$PJSIP_CONF"
sed -i "s|TELNYX_SIP_PASS_PLACEHOLDER|${TELNYX_SIP_PASS_ESCAPED}|g" "$PJSIP_CONF"
sed -i "s|TELNYX_SIP_DOMAIN_PLACEHOLDER|${TELNYX_SIP_DOMAIN_ESCAPED}|g" "$PJSIP_CONF"
sed -i "s|TELNYX_OUTBOUND_CID_PLACEHOLDER|${TELNYX_OUTBOUND_CID_ESCAPED}|g" "$EXTENSIONS_CONF"
sed -i "s|TELNYX_OUTBOUND_NAME_PLACEHOLDER|${TELNYX_OUTBOUND_NAME_ESCAPED}|g" "$EXTENSIONS_CONF"

echo "Telnyx telephony config populated."

# --- Create session log directory ---
mkdir -p /var/log/oob-sessions
chmod 1777 /var/log/oob-sessions

# --- Start D-Modem (slmodemd + d-modem) ---
DEVICE_PATH=${DEVICE_PATH:-/dev/ttySL0}

echo "Starting D-Modem..."
slmodemd -e /usr/local/bin/d-modem &
DMODEM_PID=$!
echo "  D-Modem started (PID ${DMODEM_PID})"

# Wait for modem device to appear
echo "Waiting for ${DEVICE_PATH}..."
for i in $(seq 1 10); do
    if [[ -e "${DEVICE_PATH}" ]]; then
        echo "  ${DEVICE_PATH} - OK"
        break
    fi
    if [ "$i" -eq 10 ]; then
        echo "  ERROR: ${DEVICE_PATH} did not appear after 10s"
        exit 1
    fi
    sleep 1
done

# --- Start Asterisk ---
echo "Starting Asterisk..."
asterisk -f &
ASTERISK_PID=$!

# Wait for Asterisk to be ready
sleep 3
if kill -0 $ASTERISK_PID 2>/dev/null; then
    echo "Asterisk running (PID ${ASTERISK_PID})."
else
    echo "ERROR: Asterisk failed to start!"
    exit 1
fi

# --- Start Go SSH server (replaces sshd) ---
echo "=== OOB Console Hub Ready ==="
echo "SSH server listening on :${SSH_PORT:-2222}"
echo "Modem device: ${DEVICE_PATH}"
echo "Manage users with: docker exec oob-console-hub oob-manage <command>"

exec /usr/local/bin/oob-hub
