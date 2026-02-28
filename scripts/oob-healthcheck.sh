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

MODEM_COUNT=${MODEM_COUNT:-8}
MODEM_DEVICE_PREFIX=${MODEM_DEVICE_PREFIX:-/dev/ttyIAX}

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

# 1. Go SSH server running
check "oob-hub running" critical \
    pgrep -x oob-hub

# 2. SSH port responding
check "SSH listening on 2222" critical \
    bash -c 'echo | timeout 5 bash -c "cat < /dev/tcp/127.0.0.1/2222" 2>/dev/null; [[ $? -ne 1 ]]'

# 3. D-Modem backend process running
check "slmodemd running" critical \
    pgrep -x slmodemd

# 4. At least one modem device exists
check "Modem devices present" critical \
    bash -c "ls ${MODEM_DEVICE_PREFIX}* >/dev/null 2>&1"

# --- Warning checks (alert but don't restart) ---

# 5. Ensure modem application processes exist.
check "d-modem process(es) running" warning \
    bash -c 'pgrep -f "/usr/local/bin/d-modem" >/dev/null 2>&1'

# 6. All expected modem devices present
check "All ${MODEM_COUNT} modem devices present" warning \
    bash -c "
        count=0
        for i in \$(seq 0 $((MODEM_COUNT - 1))); do
            [[ -e ${MODEM_DEVICE_PREFIX}\${i} ]] && count=\$((count + 1))
        done
        [[ \$count -eq ${MODEM_COUNT} ]]
    "

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
