#!/bin/bash
# Start D-Modem instances and expose stable modem device symlinks.

set -euo pipefail

MODEM_COUNT=${MODEM_COUNT:-8}
MODEM_DEVICE_PREFIX=${MODEM_DEVICE_PREFIX:-/dev/ttyIAX}
DMODEM_PTY_DIR=${DMODEM_PTY_DIR:-/tmp}
DMODEM_MODEMD_BIN=${DMODEM_MODEMD_BIN:-/usr/local/bin/slmodemd}
DMODEM_APP_BIN=${DMODEM_APP_BIN:-/usr/local/bin/d-modem.nopulse}
DMODEM_MODEMD_ARGS=${DMODEM_MODEMD_ARGS:-}
DMODEM_STARTUP_TIMEOUT_SEC=${DMODEM_STARTUP_TIMEOUT_SEC:-8}

if [[ ! -x "${DMODEM_MODEMD_BIN}" ]]; then
    echo "ERROR: ${DMODEM_MODEMD_BIN} is not executable"
    exit 1
fi

if [[ ! -x "${DMODEM_APP_BIN}" ]]; then
    echo "ERROR: ${DMODEM_APP_BIN} is not executable"
    exit 1
fi

mkdir -p /var/log/dmodem "${DMODEM_PTY_DIR}"

echo "Starting ${MODEM_COUNT} D-Modem instances..."

for i in $(seq 0 $((MODEM_COUNT - 1))); do
    pty_path="${DMODEM_PTY_DIR}/ttySL${i}"
    device_path="${MODEM_DEVICE_PREFIX}${i}"
    logfile="/var/log/dmodem/slmodemd-${i}.log"

    # Clear stale links from previous runs.
    rm -f "${pty_path}" "${device_path}"

    modemd_args=()
    if [[ -n "${DMODEM_MODEMD_ARGS}" ]]; then
        read -r -a modemd_args <<< "${DMODEM_MODEMD_ARGS}"
    fi

    "${DMODEM_MODEMD_BIN}" "${modemd_args[@]}" -e "${DMODEM_APP_BIN}" "${pty_path}" >"${logfile}" 2>&1 &
    pid=$!
    echo "  Started D-Modem ${i} (PID ${pid}) -> ${pty_path}"

    # Wait for slmodemd to create the PTY link, then expose the stable modem path.
    deadline=$((SECONDS + DMODEM_STARTUP_TIMEOUT_SEC))
    while [[ ${SECONDS} -lt ${deadline} ]]; do
        if [[ -e "${pty_path}" ]]; then
            ln -s "${pty_path}" "${device_path}"
            break
        fi
        sleep 0.2
    done

    if [[ ! -e "${pty_path}" ]]; then
        echo "  WARNING: ${pty_path} not created within ${DMODEM_STARTUP_TIMEOUT_SEC}s (see ${logfile})"
    fi
done

echo "D-Modem startup complete."
