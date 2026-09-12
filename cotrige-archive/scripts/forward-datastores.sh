#!/usr/bin/env bash

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

KUBECONFIG_PATH="${KUBECONFIG_PATH:-${REPO_ROOT}/kind-host}"
NAMESPACE="${NAMESPACE:-cotriage}"
MYSQL_LOCAL_PORT="${MYSQL_LOCAL_PORT:-3306}"
REDIS_LOCAL_PORT="${REDIS_LOCAL_PORT:-6379}"
ONNX_CLASSIFIER_LOCAL_PORT="${ONNX_CLASSIFIER_LOCAL_PORT:-18080}"
CHECK_INTERVAL="${CHECK_INTERVAL:-60}"
KUBE_REQUEST_TIMEOUT="${KUBE_REQUEST_TIMEOUT:-30s}"
FORWARD_READY_TIMEOUT="${FORWARD_READY_TIMEOUT:-30}"

mysql_pid=""
redis_pid=""
onnx_classifier_pid=""

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

cleanup() {
  local pid
  log "Stopping port-forwards..."
  for pid in "${mysql_pid}" "${redis_pid}" "${onnx_classifier_pid}"; do
    if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
      kill "${pid}" 2>/dev/null || true
      wait "${pid}" 2>/dev/null || true
    fi
  done
}

trap cleanup EXIT
trap 'exit 0' INT TERM

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: required command not found: $1" >&2
    exit 1
  fi
}

port_is_reachable() {
  nc -z -w 2 127.0.0.1 "$1" >/dev/null 2>&1
}

stop_owned_forward() {
  local pid="$1"
  if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
    kill "${pid}" 2>/dev/null || true
    wait "${pid}" 2>/dev/null || true
  fi
}

reconcile_forward() {
  local service="$1"
  local local_port="$2"
  local remote_port="$3"
  local pid_variable="$4"
  local current_pid="${!pid_variable:-}"
  local new_pid
  local attempt

  if port_is_reachable "${local_port}"; then
    if [[ -n "${current_pid}" ]] && ! kill -0 "${current_pid}" 2>/dev/null; then
      printf -v "${pid_variable}" '%s' ""
    fi
    log "[ok] ${service} is reachable at 127.0.0.1:${local_port}"
    return
  fi

  log "[down] ${service} is unreachable at 127.0.0.1:${local_port}"
  stop_owned_forward "${current_pid}"
  printf -v "${pid_variable}" '%s' ""

  if ! kubectl --request-timeout="${KUBE_REQUEST_TIMEOUT}" --kubeconfig "${KUBECONFIG_PATH}" -n "${NAMESPACE}" get "service/${service}" >/dev/null 2>&1; then
    log "[retry] service/${service} is unavailable; retry in ${CHECK_INTERVAL}s"
    return
  fi

  log "[start] ${service}: 127.0.0.1:${local_port} -> service/${service}:${remote_port}"
  kubectl --request-timeout="${KUBE_REQUEST_TIMEOUT}" --kubeconfig "${KUBECONFIG_PATH}" -n "${NAMESPACE}" \
    port-forward --address 127.0.0.1 "service/${service}" "${local_port}:${remote_port}" &
  new_pid=$!
  printf -v "${pid_variable}" '%s' "${new_pid}"

  for attempt in $(seq 1 "${FORWARD_READY_TIMEOUT}"); do
    sleep 1
    if port_is_reachable "${local_port}"; then
      log "[ready] ${service} port-forward started (pid=${new_pid})"
      return
    fi
    if ! kill -0 "${new_pid}" 2>/dev/null; then
      break
    fi
  done

  log "[retry] ${service} port-forward failed to become ready; retry in ${CHECK_INTERVAL}s"
  stop_owned_forward "${new_pid}"
  printf -v "${pid_variable}" '%s' ""
}

require_command kubectl
require_command nc
require_command seq

if [[ ! -f "${KUBECONFIG_PATH}" ]]; then
  echo "Error: kubeconfig not found: ${KUBECONFIG_PATH}" >&2
  exit 1
fi

if ! [[ "${CHECK_INTERVAL}" =~ ^[1-9][0-9]*$ ]]; then
  echo "Error: CHECK_INTERVAL must be a positive integer" >&2
  exit 1
fi
if ! [[ "${FORWARD_READY_TIMEOUT}" =~ ^[1-9][0-9]*$ ]]; then
  echo "Error: FORWARD_READY_TIMEOUT must be a positive integer" >&2
  exit 1
fi

log "Starting datastore port monitor (interval=${CHECK_INTERVAL}s, ready_timeout=${FORWARD_READY_TIMEOUT}s)"
log "Press Ctrl-C to stop port-forwards started by this script."

while true; do
  reconcile_forward mysql "${MYSQL_LOCAL_PORT}" 3306 mysql_pid
  reconcile_forward redis "${REDIS_LOCAL_PORT}" 6379 redis_pid
  reconcile_forward onnx-classifier "${ONNX_CLASSIFIER_LOCAL_PORT}" 18080 onnx_classifier_pid
  sleep "${CHECK_INTERVAL}"
done
