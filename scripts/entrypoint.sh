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
    echo "SIP authentication will fail for outbound modem calls."
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

MODEM_COUNT=${MODEM_COUNT:-8}
MODEM_BACKEND=${MODEM_BACKEND:-dmodem}
MODEM_DEVICE_PREFIX=${MODEM_DEVICE_PREFIX:-/dev/ttyIAX}

case "${MODEM_BACKEND}" in
    iaxmodem)
        # --- Generate IAXmodem configs ---
        /usr/local/bin/setup-iaxmodem.sh

        # --- Start IAXmodem instances ---
        echo "Starting ${MODEM_COUNT} IAXmodem instances..."
        for i in $(seq 0 $((MODEM_COUNT - 1))); do
            iaxmodem "ttyIAX${i}" &
            echo "  Started IAXmodem ttyIAX${i} (PID $!)"
        done
        sleep 2
        ;;
    dmodem)
        # D-Modem uses SIP directly; provide SIP_LOGIN expected by d-modem.
        if [[ -n "${TELNYX_SIP_USER:-}" && -n "${TELNYX_SIP_PASS:-}" ]]; then
            export SIP_LOGIN="${TELNYX_SIP_USER}:${TELNYX_SIP_PASS}@${TELNYX_SIP_DOMAIN:-sip.telnyx.com}"
        else
            echo "WARNING: TELNYX_SIP_USER/TELNYX_SIP_PASS missing; dmodem calls will fail."
        fi

        /usr/local/bin/start-dmodem.sh
        ;;
    *)
        echo "ERROR: Unsupported MODEM_BACKEND='${MODEM_BACKEND}' (expected 'dmodem' or 'iaxmodem')."
        exit 1
        ;;
esac

# Verify modem devices
echo "Modem devices:"
for i in $(seq 0 $((MODEM_COUNT - 1))); do
    dev="${MODEM_DEVICE_PREFIX}${i}"
    if [[ -e "${dev}" ]]; then
        echo "  ${dev} - OK"
    else
        echo "  ${dev} - MISSING (${MODEM_BACKEND} may have failed)"
    fi
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
echo "Manage users with: docker exec oob-console-hub oob-manage <command>"

exec /usr/local/bin/oob-hub
