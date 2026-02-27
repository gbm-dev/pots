#!/bin/bash
# OOB Console Hub - Watchdog
# Runs on the HOST via systemd timer every 2 minutes.
# Checks container health and takes corrective action.

set -uo pipefail

CONTAINER="oob-console-hub"
LOG_TAG="oob-watchdog"
MAX_RESTARTS_PER_HOUR=3
STATE_DIR="/var/lib/oob-watchdog"
RESTART_LOG="${STATE_DIR}/restarts.log"

mkdir -p "$STATE_DIR"

log() {
    logger -t "$LOG_TAG" "$1"
    echo "$(date '+%Y-%m-%d %H:%M:%S') $1"
}

restart_service() {
    local reason="$1"
    log "RESTARTING: ${reason}"

    # Check restart rate limit
    local recent_restarts
    recent_restarts=$(find "$RESTART_LOG" -mmin -60 2>/dev/null | wc -l)

    if [[ -f "$RESTART_LOG" ]]; then
        recent_restarts=$(awk -v cutoff="$(date -d '1 hour ago' '+%s' 2>/dev/null || date -v-1H '+%s')" \
            '$1 > cutoff { count++ } END { print count+0 }' "$RESTART_LOG")
    fi

    if [[ ${recent_restarts:-0} -ge $MAX_RESTARTS_PER_HOUR ]]; then
        log "RATE LIMITED: ${recent_restarts} restarts in the last hour (max ${MAX_RESTARTS_PER_HOUR}). Manual intervention required."
        return 1
    fi

    # Record this restart
    echo "$(date '+%s') ${reason}" >> "$RESTART_LOG"

    # Prune old entries (keep last 24h)
    if [[ -f "$RESTART_LOG" ]]; then
        local cutoff
        cutoff=$(date -d '24 hours ago' '+%s' 2>/dev/null || date -v-24H '+%s')
        awk -v c="$cutoff" '$1 > c' "$RESTART_LOG" > "${RESTART_LOG}.tmp" && mv "${RESTART_LOG}.tmp" "$RESTART_LOG"
    fi

    systemctl reload oob-hub.service
    sleep 10

    # Verify it came back
    if docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null | grep -q "true"; then
        log "RECOVERED: Service restarted successfully."
    else
        log "FAILED: Service did not recover after restart."
    fi
}

# --- Check 1: Container running? ---
if ! docker inspect "$CONTAINER" &>/dev/null; then
    restart_service "Container does not exist"
    exit 0
fi

CONTAINER_STATE=$(docker inspect -f '{{.State.Status}}' "$CONTAINER" 2>/dev/null)

if [[ "$CONTAINER_STATE" != "running" ]]; then
    restart_service "Container state: ${CONTAINER_STATE}"
    exit 0
fi

# --- Check 2: Run health check inside container ---
HEALTH_OUTPUT=$(docker exec "$CONTAINER" /usr/local/bin/oob-healthcheck.sh --verbose 2>&1)
HEALTH_EXIT=$?

case $HEALTH_EXIT in
    0)
        # Healthy - nothing to do
        ;;
    1)
        # Critical failure
        log "UNHEALTHY: Critical failure detected"
        log "Health output: ${HEALTH_OUTPUT}"
        restart_service "Health check critical failure"
        ;;
    2)
        # Degraded - log warning but don't restart
        log "DEGRADED: ${HEALTH_OUTPUT}"
        ;;
    *)
        log "UNKNOWN: Health check returned exit code ${HEALTH_EXIT}"
        ;;
esac

# --- Check 3: Container resource sanity ---
# Check if container is consuming excessive memory (>90% of its limit)
MEM_USAGE=$(docker stats --no-stream --format '{{.MemPerc}}' "$CONTAINER" 2>/dev/null | tr -d '%')
if [[ -n "$MEM_USAGE" ]] && awk "BEGIN{exit !($MEM_USAGE > 90)}" 2>/dev/null; then
    log "WARNING: Container memory usage at ${MEM_USAGE}%"
fi

# --- Check 4: SSH port actually accepting connections from host ---
if ! timeout 5 bash -c "echo >/dev/tcp/127.0.0.1/2222" 2>/dev/null; then
    log "UNHEALTHY: SSH port 2222 not responding from host"
    restart_service "SSH port unreachable from host"
fi
