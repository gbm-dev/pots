#!/bin/bash
# OOB Console Hub - Local Development Infrastructure
# Starts D-Modem and Asterisk natively (no Docker).
# Run oob-probe or oob-hub separately in another terminal.
#
# Usage:
#   ./scripts/local-dev.sh              # start infra
#   go run ./cmd/oob-probe -dial 15551234567  # in another terminal
#
# Asterisk diagnostic commands (while running):
#   ./scripts/local-dev.sh ast "pjsip show registrations"
#   ./scripts/local-dev.sh ast "dialplan show oob-outbound"
#   ./scripts/local-dev.sh ast "pjsip show endpoints"

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Source .env if present
if [[ -f "${PROJECT_DIR}/.env" ]]; then
    echo "Loading .env..."
    set -a
    source "${PROJECT_DIR}/.env"
    set +a
fi

DEVICE_PATH="${DEVICE_PATH:-/dev/ttySL0}"
DMODEM_BIN="${DMODEM_BIN:-${PROJECT_DIR}/bin/d-modem}"
SLMODEMD_BIN="${SLMODEMD_BIN:-${PROJECT_DIR}/bin/slmodemd}"
LOG_DIR="${LOG_DIR:-${PROJECT_DIR}/logs}"
AST_LOG="${LOG_DIR}/asterisk.log"
AST_RUNTIME_CONF_FILE="${LOG_DIR}/.local-dev-asterisk.conf"

export DEVICE_PATH LOG_DIR

mkdir -p "$LOG_DIR"

# Resolve and use the active temp asterisk.conf when running CLI commands.
ast_cli() {
    local cmd="$*"
    local conf_path="${AST_CONF_PATH:-}"

    if [[ -z "$conf_path" && -f "${AST_RUNTIME_CONF_FILE}" ]]; then
        conf_path="$(<"${AST_RUNTIME_CONF_FILE}")"
    fi

    if [[ -n "$conf_path" && -f "$conf_path" ]]; then
        asterisk -C "$conf_path" -rx "$cmd"
        return
    fi

    asterisk -rx "$cmd"
}

# --- Subcommand: run Asterisk CLI command ---
if [[ "${1:-}" == "ast" ]]; then
    shift
    AST_CMD="$*"
    if [[ -f "${AST_RUNTIME_CONF_FILE}" ]]; then
        AST_CONF_PATH="$(<"${AST_RUNTIME_CONF_FILE}")"
        if [[ -n "${AST_CONF_PATH}" && -f "${AST_CONF_PATH}" ]]; then
            exec asterisk -C "${AST_CONF_PATH}" -rx "${AST_CMD}"
        fi
    fi
    exec asterisk -rx "${AST_CMD}"
fi

# --- Verify binaries exist (Download if missing) ---
DMODEM_VERSION="v0.1.2"
DMODEM_BASE_URL="https://github.com/gbm-dev/D-Modem/releases/download/${DMODEM_VERSION}"

mkdir -p "${PROJECT_DIR}/bin"

for bin_path in "$SLMODEMD_BIN" "$DMODEM_BIN"; do
    if [[ ! -x "$bin_path" ]]; then
        bin_name=$(basename "$bin_path")
        echo "Binary $bin_name not found, downloading $DMODEM_VERSION..."
        curl -L -o "$bin_path" "${DMODEM_BASE_URL}/${bin_name}"
        chmod +x "$bin_path"
    fi
done

# --- Cleanup on exit ---
PIDS=()
cleanup() {
    echo ""
    echo "Shutting down..."
    for pid in "${PIDS[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            sudo kill "$pid" 2>/dev/null || kill "$pid" 2>/dev/null || true
            wait "$pid" 2>/dev/null || true
        fi
    done
    rm -f "${AST_RUNTIME_CONF_FILE}"
    if [[ -n "${ASTERISK_TMP:-}" && -d "${ASTERISK_TMP}" ]]; then
        rm -rf "${ASTERISK_TMP}"
    fi
    echo "Done."
}
trap cleanup EXIT INT TERM

# --- Generate Asterisk configs in a temp directory ---
ASTERISK_TMP=$(mktemp -d)
echo "Generating Asterisk configs in ${ASTERISK_TMP}..."

for f in "${PROJECT_DIR}/config/asterisk/"*.conf; do
    cp "$f" "${ASTERISK_TMP}/$(basename "$f")"
done

# Generate a minimal asterisk.conf pointing at our temp config dir
cat > "${ASTERISK_TMP}/asterisk.conf" <<ASTEOF
[directories]
astetcdir => ${ASTERISK_TMP}
astlogdir => ${LOG_DIR}
astrundir => ${ASTERISK_TMP}/run
astspooldir => ${ASTERISK_TMP}/spool

[options]
verbose = 5
debug = 2
ASTEOF

mkdir -p "${ASTERISK_TMP}/run" "${ASTERISK_TMP}/spool"

# Substitute Telnyx credentials
if [[ -n "${TELNYX_SIP_USER:-}" ]]; then
    sed -i "s|TELNYX_SIP_USER_PLACEHOLDER|${TELNYX_SIP_USER}|g" "${ASTERISK_TMP}/pjsip_wizard.conf"
    sed -i "s|TELNYX_SIP_PASS_PLACEHOLDER|${TELNYX_SIP_PASS:-}|g" "${ASTERISK_TMP}/pjsip_wizard.conf"
    sed -i "s|TELNYX_SIP_DOMAIN_PLACEHOLDER|${TELNYX_SIP_DOMAIN:-sip.telnyx.com}|g" "${ASTERISK_TMP}/pjsip_wizard.conf"
    sed -i "s|TELNYX_OUTBOUND_CID_PLACEHOLDER|${TELNYX_OUTBOUND_CID:-}|g" "${ASTERISK_TMP}/extensions.conf"
    sed -i "s|TELNYX_OUTBOUND_NAME_PLACEHOLDER|${TELNYX_OUTBOUND_NAME:-OOB-Console-Hub}|g" "${ASTERISK_TMP}/extensions.conf"
    echo "  Telnyx credentials substituted."
else
    echo "  WARNING: TELNYX_SIP_USER not set — trunk will not register."
fi

# --- Start D-Modem (slmodemd + d-modem) ---
echo "Starting D-Modem..."
# Limit file descriptors to 1024 to avoid FD_SETSIZE crash in 32-bit slmodemd
sudo sh -c "ulimit -n 1024; \"$SLMODEMD_BIN\" -d9 -e \"$DMODEM_BIN\"" &
PIDS+=($!)
echo "  slmodemd PID: ${PIDS[-1]}"

echo "Waiting for ${DEVICE_PATH}..."
for i in $(seq 1 10); do
    if [[ -e "${DEVICE_PATH}" ]]; then
        echo "  ${DEVICE_PATH} ready."
        break
    fi
    if [ "$i" -eq 10 ]; then
        echo "  ERROR: ${DEVICE_PATH} did not appear after 10s"
        exit 1
    fi
    sleep 1
done

# --- Start Asterisk ---
echo "Starting Asterisk (verbose=5, debug=2)..."
AST_CONF_PATH="${ASTERISK_TMP}/asterisk.conf"
printf '%s\n' "${AST_CONF_PATH}" > "${AST_RUNTIME_CONF_FILE}"
asterisk -f -C "${AST_CONF_PATH}" &>"${AST_LOG}" &
PIDS+=($!)
echo "  Asterisk PID: ${PIDS[-1]}"
echo "  Asterisk log: ${AST_LOG}"

sleep 3
if ! kill -0 "${PIDS[-1]}" 2>/dev/null; then
    echo "ERROR: Asterisk failed to start! Check ${AST_LOG}"
    tail -20 "$AST_LOG"
    exit 1
fi

# --- Enable SIP debug logging ---
ast_cli "pjsip set logger on" 2>/dev/null || true

# --- Dump initial diagnostics ---
echo ""
echo "=== Asterisk Diagnostics ==="

echo ""
echo "--- Dialplan: oob-outbound ---"
ast_cli "dialplan show oob-outbound" 2>/dev/null || echo "  (not loaded)"

echo ""
echo "--- PJSIP Endpoints ---"
ast_cli "pjsip show endpoints" 2>/dev/null || echo "  (not available)"

echo ""
echo "--- PJSIP Registrations ---"
ast_cli "pjsip show registrations" 2>/dev/null || echo "  (none)"

echo ""
echo "=== Infrastructure Ready ==="
echo "  Modem:        ${DEVICE_PATH}"
echo "  Logs:         ${LOG_DIR}"
echo "  Asterisk log: ${AST_LOG}"
echo ""
echo "Quick commands:"
echo "  go run ./cmd/oob-probe -dial <number>                    # headless test"
echo "  ./scripts/local-call-test.sh                              # fixed non-TUI test (+17186945647 @ 9600)"
echo "  go run ./cmd/oob-hub                                      # full TUI"
echo "  ./scripts/local-dev.sh ast 'pjsip show registrations'    # asterisk CLI"
echo "  ./scripts/local-dev.sh ast 'core show channels'          # active calls"
echo "  tail -f ${AST_LOG}                                        # live asterisk log"
echo ""
echo "Press Ctrl+C to stop."

# Wait for signal
wait
