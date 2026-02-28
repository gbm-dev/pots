#!/bin/bash
# OOB Console Hub - Container Entrypoint
# Starts D-Modem instances and launches the Go SSH server.

set -euo pipefail

echo "=== OOB Console Hub Starting ==="

if [[ -z "${TELNYX_SIP_USER:-}" || -z "${TELNYX_SIP_PASS:-}" ]]; then
    echo "WARNING: TELNYX_SIP_USER or TELNYX_SIP_PASS not set!"
    echo "SIP authentication will fail for outbound modem calls."
fi

# --- Create session log directory ---
mkdir -p /var/log/oob-sessions
chmod 1777 /var/log/oob-sessions

MODEM_COUNT=${MODEM_COUNT:-8}
MODEM_DEVICE_PREFIX=${MODEM_DEVICE_PREFIX:-/dev/ttyIAX}

# D-Modem uses SIP directly; provide SIP_LOGIN expected by d-modem.
if [[ -n "${TELNYX_SIP_USER:-}" && -n "${TELNYX_SIP_PASS:-}" ]]; then
    export SIP_LOGIN="${TELNYX_SIP_USER}:${TELNYX_SIP_PASS}@${TELNYX_SIP_DOMAIN:-sip.telnyx.com}"
else
    echo "WARNING: TELNYX_SIP_USER/TELNYX_SIP_PASS missing; dmodem calls will fail."
fi

/usr/local/bin/start-dmodem.sh

# Verify modem devices
echo "Modem devices:"
for i in $(seq 0 $((MODEM_COUNT - 1))); do
    dev="${MODEM_DEVICE_PREFIX}${i}"
    if [[ -e "${dev}" ]]; then
        echo "  ${dev} - OK"
    else
        echo "  ${dev} - MISSING (dmodem startup may have failed)"
    fi
done

# --- Start Go SSH server (replaces sshd) ---
echo "=== OOB Console Hub Ready ==="
echo "SSH server listening on :${SSH_PORT:-2222}"
echo "Manage users with: docker exec oob-console-hub oob-manage <command>"

exec /usr/local/bin/oob-hub
