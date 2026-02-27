#!/bin/bash
# OOB Console Hub - Health Check
# Used by: Docker HEALTHCHECK, systemd watchdog timer, manual diagnostics
#
# Exit codes:
#   0 = healthy
#   1 = unhealthy (should trigger restart)
#   2 = degraded (service works but something is wrong, alert only)
#
# When run with --verbose, prints diagnostics. Otherwise silent on success.

VERBOSE=false
[[ "${1:-}" == "--verbose" ]] && VERBOSE=true

FAILURES=0
WARNINGS=0
STATUS_LINES=()

check() {
    local name="$1" severity="$2"
    shift 2
    if "$@" >/dev/null 2>&1; then
        $VERBOSE && STATUS_LINES+=("  [OK]   $name")
    else
        if [[ "$severity" == "critical" ]]; then
            FAILURES=$((FAILURES + 1))
            STATUS_LINES+=("  [FAIL] $name")
        else
            WARNINGS=$((WARNINGS + 1))
            STATUS_LINES+=("  [WARN] $name")
        fi
    fi
}

# --- Critical checks (trigger restart) ---

# 1. SSHD responding
check "SSHD listening" critical \
    bash -c 'echo | timeout 5 bash -c "cat < /dev/tcp/127.0.0.1/22" 2>/dev/null; [[ $? -ne 1 ]]'

# 2. Asterisk running
check "Asterisk running" critical \
    asterisk -rx "core show version"

# 3. At least one IAXmodem device exists
check "IAXmodem devices present" critical \
    bash -c 'ls /dev/ttyIAX* >/dev/null 2>&1'

# --- Warning checks (alert but don't restart) ---

# 4. Telnyx SIP trunk registered
check "Telnyx trunk registered" warning \
    bash -c 'asterisk -rx "pjsip show registrations" 2>/dev/null | grep -qi "registered"'

# 5. All expected modem devices present
MODEM_COUNT=${MODEM_COUNT:-8}
check "All ${MODEM_COUNT} modem devices present" warning \
    bash -c "
        count=0
        for i in \$(seq 0 $((MODEM_COUNT - 1))); do
            [[ -e /dev/ttyIAX\${i} ]] && count=\$((count + 1))
        done
        [[ \$count -eq ${MODEM_COUNT} ]]
    "

# 6. No zombie modem locks (fuser reports busy but no screen session using it)
check "No zombie modem locks" warning \
    bash -c '
        for dev in /dev/ttyIAX*; do
            [[ ! -e "$dev" ]] && continue
            if fuser -s "$dev" 2>/dev/null; then
                pid=$(fuser "$dev" 2>/dev/null | tr -d " ")
                if [[ -n "$pid" ]] && ! ps -p "$pid" -o comm= 2>/dev/null | grep -qE "screen|expect|minicom"; then
                    exit 1  # zombie lock found
                fi
            fi
        done
        exit 0
    '

# --- Output ---
if $VERBOSE; then
    echo "OOB Health Check - $(date '+%Y-%m-%d %H:%M:%S')"
    echo "───────────────────────────────────────"
    for line in "${STATUS_LINES[@]}"; do
        echo "$line"
    done
    echo "───────────────────────────────────────"
    echo "  Critical failures: ${FAILURES}"
    echo "  Warnings:          ${WARNINGS}"
fi

if [[ $FAILURES -gt 0 ]]; then
    $VERBOSE && echo "  Status: UNHEALTHY"
    exit 1
elif [[ $WARNINGS -gt 0 ]]; then
    $VERBOSE && echo "  Status: DEGRADED"
    exit 2
else
    $VERBOSE && echo "  Status: HEALTHY"
    exit 0
fi
