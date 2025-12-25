#!/bin/sh
set -eu

ENV_FILE="/etc/binance-pivot-monitor/binance-pivot-monitor.env"
if [ -f "${ENV_FILE}" ]; then
  . "${ENV_FILE}"
fi

ADDR=${ADDR:-":8080"}
DATA_DIR=${DATA_DIR:-"/var/lib/binance-pivot-monitor"}
CORS_ORIGINS=${CORS_ORIGINS:-"*"}
BINANCE_REST=${BINANCE_REST:-"https://fapi.binance.com"}
REFRESH_WORKERS=${REFRESH_WORKERS:-"16"}
MONITOR_HEARTBEAT=${MONITOR_HEARTBEAT:-""}
HISTORY_MAX=${HISTORY_MAX:-"20000"}
HISTORY_FILE=${HISTORY_FILE:-"signals/history.jsonl"}
RESTART_SEC=${RESTART_SEC:-"2"}
EXTRA_ARGS=${EXTRA_ARGS:-""}

BIN="/usr/bin/binance-pivot-monitor"

mkdir -p "${DATA_DIR}"

child=""
term() {
  if [ -n "${child}" ]; then
    kill "${child}" 2>/dev/null || true
    wait "${child}" 2>/dev/null || true
  fi
  exit 0
}
trap term INT TERM

while true; do
  set -- \
    -addr "${ADDR}" \
    -data-dir "${DATA_DIR}" \
    -cors-origins "${CORS_ORIGINS}" \
    -binance-rest "${BINANCE_REST}" \
    -refresh-workers "${REFRESH_WORKERS}" \
    -history-max "${HISTORY_MAX}" \
    -history-file "${HISTORY_FILE}"

  if [ -n "${MONITOR_HEARTBEAT}" ]; then
    set -- "$@" -monitor-heartbeat "${MONITOR_HEARTBEAT}"
  fi

  if [ -n "${EXTRA_ARGS}" ]; then
    set -- "$@" ${EXTRA_ARGS}
  fi

  "${BIN}" "$@" &
  child=$!

  set +e
  wait "${child}"
  code=$?
  set -e

  child=""

  echo "binance-pivot-monitor exited with code ${code}, restarting in ${RESTART_SEC}s..." >&2
  sleep "${RESTART_SEC}"
done
