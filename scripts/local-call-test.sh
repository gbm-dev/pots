#!/bin/bash
# Fixed non-TUI local call test:
# - Number: +17186945647
# - Baud:   9600
#
# Produces a single timestamped results file under logs/probe-runs/.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

DIAL_NUMBER="+17186945647"
MODEM_INIT="AT+MS=132,0,9600,9600"
PROBE_TIMEOUT="${PROBE_TIMEOUT:-75s}"
ENTER_INTERVAL="${ENTER_INTERVAL:-2s}"
READY_TIMEOUT_SEC="${READY_TIMEOUT_SEC:-45}"
REGISTER_TIMEOUT_SEC="${REGISTER_TIMEOUT_SEC:-45}"

LOG_DIR="${LOG_DIR:-${PROJECT_DIR}/logs}"
RESULTS_DIR="${LOG_DIR}/probe-runs"
AST_RUNTIME_CONF_FILE="${LOG_DIR}/.local-dev-asterisk.conf"
TS="$(date +%Y%m%d-%H%M%S)"
RESULT_FILE="${RESULTS_DIR}/fixed-call-${TS}.log"
STACK_LOG="${RESULTS_DIR}/local-dev-${TS}.log"

mkdir -p "${LOG_DIR}" "${RESULTS_DIR}"

started_stack=0
local_dev_pid=""

run_ast() {
    ./scripts/local-dev.sh ast "$1" 2>&1
}

stack_ready() {
    local conf_path
    if [[ ! -f "${AST_RUNTIME_CONF_FILE}" ]]; then
        return 1
    fi
    conf_path="$(<"${AST_RUNTIME_CONF_FILE}")"
    if [[ -z "${conf_path}" || ! -f "${conf_path}" ]]; then
        return 1
    fi
    run_ast "pjsip show endpoint dmodem" | grep -q "Endpoint:  dmodem"
}

is_registered() {
    local out
    out="$(run_ast "pjsip show registrations" || true)"
    echo "$out" | grep -Eq 'telnyx-out-reg-0/.*[[:space:]]Registered[[:space:]]+\(exp\.'
}

cleanup() {
    if [[ "${started_stack}" -eq 1 && -n "${local_dev_pid}" ]]; then
        if kill -0 "${local_dev_pid}" 2>/dev/null; then
            {
                echo ""
                echo "[cleanup] Stopping local-dev stack pid=${local_dev_pid}"
            } | tee -a "${RESULT_FILE}"
            kill "${local_dev_pid}" 2>/dev/null || true
            wait "${local_dev_pid}" 2>/dev/null || true
        fi
    fi
}
trap cleanup EXIT INT TERM

{
    echo "=== Fixed Non-TUI Local Call Test ==="
    echo "timestamp: $(date --iso-8601=seconds)"
    echo "dial_number: ${DIAL_NUMBER}"
    echo "baud: 9600"
    echo "modem_init: ${MODEM_INIT}"
    echo "device_path: ${DEVICE_PATH:-/dev/ttySL0}"
    echo "result_file: ${RESULT_FILE}"
} | tee "${RESULT_FILE}"

if [[ "${SLMODEMD_BIN:-}" == "/usr/sbin/slmodemd" ]]; then
    {
        echo "[error] SLMODEMD_BIN is set to /usr/sbin/slmodemd."
        echo "[error] That distro binary does not support -e <d-modem> and will fail in local-dev."
        echo "[error] Unset SLMODEMD_BIN or set it to ${PROJECT_DIR}/bin/slmodemd."
    } | tee -a "${RESULT_FILE}"
    exit 2
fi

if stack_ready; then
    echo "[info] Reusing running local-dev stack." | tee -a "${RESULT_FILE}"
else
    echo "[info] Starting local-dev stack..." | tee -a "${RESULT_FILE}"
    ./scripts/local-dev.sh >"${STACK_LOG}" 2>&1 &
    local_dev_pid="$!"
    started_stack=1
    echo "[info] local-dev startup log: ${STACK_LOG}" | tee -a "${RESULT_FILE}"

    deadline=$((SECONDS + READY_TIMEOUT_SEC))
    while (( SECONDS < deadline )); do
        if stack_ready; then
            break
        fi
        if ! kill -0 "${local_dev_pid}" 2>/dev/null; then
            {
                echo "[error] local-dev exited before becoming ready."
                echo "[error] tail of startup log:"
                tail -n 80 "${STACK_LOG}" || true
            } | tee -a "${RESULT_FILE}"
            exit 1
        fi
        sleep 1
    done

    if ! stack_ready; then
        {
            echo "[error] local-dev was not ready within ${READY_TIMEOUT_SEC}s."
            echo "[error] tail of startup log:"
            tail -n 80 "${STACK_LOG}" || true
        } | tee -a "${RESULT_FILE}"
        exit 1
    fi
fi

echo "[info] Waiting for Telnyx registration..." | tee -a "${RESULT_FILE}"
reg_deadline=$((SECONDS + REGISTER_TIMEOUT_SEC))
registration_ready=0
while (( SECONDS < reg_deadline )); do
    if is_registered; then
        echo "[info] Telnyx registration is Registered." | tee -a "${RESULT_FILE}"
        registration_ready=1
        break
    fi
    sleep 1
done

if [[ "${registration_ready}" -ne 1 ]]; then
    {
        echo "[error] Registration did not reach Registered in ${REGISTER_TIMEOUT_SEC}s."
        echo ">>> pjsip show registrations"
        run_ast "pjsip show registrations" || true
    } | tee -a "${RESULT_FILE}"
    exit 1
fi

{
    echo ""
    echo "=== Pre-Call Diagnostics ==="
    echo ""
    echo ">>> pjsip show registrations"
    run_ast "pjsip show registrations" || true
    echo ""
    echo ">>> pjsip show endpoints"
    run_ast "pjsip show endpoints" || true
    echo ""
    echo "=== Running oob-probe ==="
    echo "command: go run ./cmd/oob-probe -dial ${DIAL_NUMBER} -init ${MODEM_INIT} -timeout ${PROBE_TIMEOUT} -enter-interval ${ENTER_INTERVAL}"
    echo ""
} | tee -a "${RESULT_FILE}"

set +e
go run ./cmd/oob-probe \
    -dial "${DIAL_NUMBER}" \
    -init "${MODEM_INIT}" \
    -timeout "${PROBE_TIMEOUT}" \
    -enter-interval "${ENTER_INTERVAL}" \
    2>&1 | tee -a "${RESULT_FILE}"
probe_rc=${PIPESTATUS[0]}
set -e

{
    echo ""
    echo "=== Post-Call Diagnostics ==="
    echo ""
    echo ">>> core show channels"
    run_ast "core show channels" || true
    echo ""
    echo ">>> pjsip show registrations"
    run_ast "pjsip show registrations" || true
    echo ""
    echo ">>> tail -n 120 ${LOG_DIR}/asterisk.log"
    tail -n 120 "${LOG_DIR}/asterisk.log" 2>/dev/null || true
    echo ""
    if [[ "${probe_rc}" -eq 0 ]]; then
        echo "RESULT: PASS (probe exit code 0)"
    else
        echo "RESULT: FAIL (probe exit code ${probe_rc})"
    fi
} | tee -a "${RESULT_FILE}"

echo "Wrote results: ${RESULT_FILE}"
if [[ "${started_stack}" -eq 1 ]]; then
    echo "Wrote local-dev startup log: ${STACK_LOG}"
fi

exit "${probe_rc}"
