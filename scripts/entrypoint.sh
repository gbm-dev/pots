#!/bin/bash
# OOB Console Hub - Container Entrypoint
# Generates configs, substitutes env vars, starts all services

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

# --- Restore persisted users ---
# User data is persisted via volume at /data/users
DATA_DIR=/data/users
mkdir -p "$DATA_DIR"

# Create the oob group if it doesn't exist
groupadd -f oob

# Restore users from persisted state
if [[ -f "${DATA_DIR}/users.conf" ]]; then
    echo "Restoring persisted users..."
    while IFS=: read -r username _; do
        if ! id "$username" &>/dev/null; then
            useradd -m -s /usr/local/bin/oob-menu -G oob,dialout "$username" 2>/dev/null || true
        fi
    done < "${DATA_DIR}/users.conf"

    # Restore shadow entries (passwords)
    if [[ -f "${DATA_DIR}/shadow.conf" ]]; then
        while IFS=: read -r username hash rest; do
            usermod -p "$hash" "$username" 2>/dev/null || true
            # Restore password expiry (force change on next login)
            if grep -q "^${username}$" "${DATA_DIR}/force_change.list" 2>/dev/null; then
                chage -d 0 "$username"
            fi
        done < "${DATA_DIR}/shadow.conf"
    fi

    echo "  Users restored."
else
    echo "No persisted users found. Use oob-user-manage to create users."
fi

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
        chmod 660 "/dev/ttyIAX${i}"
        chgrp oob "/dev/ttyIAX${i}"
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

# --- Setup and start SSH ---
echo "Configuring SSH..."
mkdir -p /run/sshd

# Ensure SSH host keys exist (persisted via volume)
if [[ -d "${DATA_DIR}/ssh_host_keys" ]]; then
    cp "${DATA_DIR}/ssh_host_keys/"* /etc/ssh/ 2>/dev/null || true
fi
ssh-keygen -A 2>/dev/null || true
mkdir -p "${DATA_DIR}/ssh_host_keys"
cp /etc/ssh/ssh_host_*_key* "${DATA_DIR}/ssh_host_keys/" 2>/dev/null || true

# Configure sshd
cat > /etc/ssh/sshd_config.d/oob.conf <<'SSHEOF'
# OOB Console Hub SSH config
PermitRootLogin no
PasswordAuthentication yes
PrintMotd yes
Banner none
ClientAliveInterval 30
ClientAliveCountMax 3

# Only allow members of oob group
AllowGroups oob
SSHEOF

# MOTD
cat > /etc/motd <<'MOTDEOF'

  ╔══════════════════════════════════════════╗
  ║     OOB Console Hub - Telnyx/Asterisk    ║
  ║                                          ║
  ║  You will be presented with a menu of    ║
  ║  available remote console connections.   ║
  ╚══════════════════════════════════════════╝

MOTDEOF

echo "Starting SSH daemon (foreground)..."
echo "=== OOB Console Hub Ready ==="
echo "Manage users with: oob-user-manage (from host)"

# Run sshd in foreground (keeps container alive)
exec /usr/sbin/sshd -D -e
