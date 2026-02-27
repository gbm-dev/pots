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

# --- Setup OOB user ---
OOB_PASSWORD=${OOB_PASSWORD:-changeme}

if ! id oob &>/dev/null; then
    useradd -m -s /usr/local/bin/oob-menu oob
fi
echo "oob:${OOB_PASSWORD}" | chpasswd

# Allow oob user to access modem devices
usermod -aG dialout oob 2>/dev/null || true

echo "OOB user configured."

# --- Generate IAXmodem configs ---
/usr/local/bin/setup-iaxmodem.sh

# --- Create session log directory ---
mkdir -p /var/log/oob-sessions
chown oob:oob /var/log/oob-sessions

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

# --- Setup and start SSH ---
echo "Configuring SSH..."
mkdir -p /run/sshd

# Ensure SSH host keys exist
ssh-keygen -A 2>/dev/null || true

# Configure sshd for our use case
cat > /etc/ssh/sshd_config.d/oob.conf <<'SSHEOF'
# OOB Console Hub SSH config
PermitRootLogin no
PasswordAuthentication yes
AllowUsers oob
PrintMotd yes
Banner none
ClientAliveInterval 30
ClientAliveCountMax 3
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
echo "SSH into this container as user 'oob' to access the console menu."

# Run sshd in foreground (keeps container alive)
exec /usr/sbin/sshd -D -e
