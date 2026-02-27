#!/bin/bash
# OOB Console Hub - Container Entrypoint
# Generates configs, substitutes env vars, starts telephony, then launches Go SSH server

set -euo pipefail

echo "=== OOB Console Hub Starting ==="

# --- Substitute Telnyx credentials into PJSIP config ---
PJSIP_CONF=/etc/asterisk/pjsip_wizard.conf

if [[ -z "${TELNYX_SIP_USER:-}" || -z "${TELNYX_SIP_PASS:-}" ]]; then
    echo "WARNING: TELNYX_SIP_USER or TELNYX_SIP_PASS not set!"
    echo "Asterisk will start but Telnyx trunk will not register."
fi

sed -i "s|TELNYX_SIP_USER_PLACEHOLDER|${TELNYX_SIP_USER:-unset}|g" "$PJSIP_CONF"
sed -i "s|TELNYX_SIP_PASS_PLACEHOLDER|${TELNYX_SIP_PASS:-unset}|g" "$PJSIP_CONF"
sed -i "s|TELNYX_SIP_DOMAIN_PLACEHOLDER|${TELNYX_SIP_DOMAIN:-sip.telnyx.com}|g" "$PJSIP_CONF"

echo "Telnyx PJSIP config populated."

# --- Create session log directory ---
mkdir -p /var/log/oob-sessions
chmod 1777 /var/log/oob-sessions

# --- Generate IAXmodem configs ---
/usr/local/bin/setup-iaxmodem.sh

# --- Start IAXmodem instances ---
MODEM_COUNT=${MODEM_COUNT:-8}
echo "Starting ${MODEM_COUNT} IAXmodem instances..."

for i in $(seq 0 $((MODEM_COUNT - 1))); do
    iaxmodem "ttyIAX${i}" &
    echo "  Started IAXmodem ttyIAX${i} (PID $!)"
done

# Give IAXmodem a moment to create PTYs
sleep 2

# Verify modem devices
echo "Modem devices:"
for i in $(seq 0 $((MODEM_COUNT - 1))); do
    if [[ -e "/dev/ttyIAX${i}" ]]; then
        echo "  /dev/ttyIAX${i} - OK"
    else
        echo "  /dev/ttyIAX${i} - MISSING (IAXmodem may have failed)"
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
