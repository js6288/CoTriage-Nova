#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load secrets and local overrides before deriving the runtime configuration.
if [[ -f "${SCRIPT_DIR}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${SCRIPT_DIR}/.env"
  set +a
fi

KUBECONFIG_PATH="${KUBECONFIG_PATH:-${SCRIPT_DIR}/kind-host}"
NAMESPACE="${NAMESPACE:-cotriage}"
RUNTIME_DIR="${COTRIAGE_RUNTIME_DIR:-${SCRIPT_DIR}/.runtime}"
LOG_DIR="${RUNTIME_DIR}/logs"

MYSQL_LOCAL_PORT="${MYSQL_LOCAL_PORT:-3306}"
REDIS_LOCAL_PORT="${REDIS_LOCAL_PORT:-6379}"
ONNX_LOCAL_PORT="${ONNX_CLASSIFIER_LOCAL_PORT:-18080}"
SERVER_LOCAL_PORT="${SERVER_LOCAL_PORT:-8080}"
WEB_LOCAL_PORT="${WEB_LOCAL_PORT:-5173}"
AVATAR_TOKEN_PORT="${AVATAR_TOKEN_PORT:-18090}"
STARTUP_TIMEOUT="${STARTUP_TIMEOUT:-180}"

MYSQL_DSN="${MYSQL_DSN:-cotriage:cotriage-pass@tcp(127.0.0.1:${MYSQL_LOCAL_PORT})/cotriage?charset=utf8mb4&parseTime=true&loc=UTC}"
WITH_AVATAR=0
WITH_CLOUD=1
WITH_WEB=1

owned_pids=""
process_names=""
started_pid=""

usage() {
  cat <<'EOF'
Usage: ./start-cotriage.sh [options]

Starts the complete CoTriage development stack. By default it:
  1. checks the Kubernetes infrastructure;
  2. clears the cotriage Redis database;
  3. restarts the cotriage-log-collector DaemonSet;
  4. forwards MySQL, Redis and ONNX classifier ports;
  5. starts server, edge-infer, router-controller, cloud-gateway and Web locally.

Options:
  --with-avatar  Also start the LiveKit token server and avatar agent worker.
  --no-cloud     Do not start cloud-gateway.
  --no-web       Do not start the Vite Web frontend.
  -h, --help     Show this help.

Environment overrides:
  KUBECONFIG_PATH, NAMESPACE, MYSQL_LOCAL_PORT, REDIS_LOCAL_PORT,
  ONNX_CLASSIFIER_LOCAL_PORT, SERVER_LOCAL_PORT, WEB_LOCAL_PORT,
  AVATAR_TOKEN_PORT, STARTUP_TIMEOUT, MYSQL_DSN, EDGE_LLM_URL,
  DEEPSEEK_API_KEY.

Press Ctrl-C to stop all local processes and port-forwards started by this script.
Logs are written to .runtime/logs/.
EOF
}

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

die() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

cleanup() {
  local pid
  trap - EXIT INT TERM
  log "Stopping CoTriage local processes and owned port-forwards..."
  for pid in ${owned_pids}; do
    if kill -0 "${pid}" 2>/dev/null; then
      kill "${pid}" 2>/dev/null || true
    fi
  done
  for pid in ${owned_pids}; do
    wait "${pid}" 2>/dev/null || true
  done
  log "Stopped. Logs remain in ${LOG_DIR}"
}

port_is_open() {
  nc -z -w 1 127.0.0.1 "$1" >/dev/null 2>&1
}

wait_for_port() {
  local name="$1"
  local port="$2"
  local pid="${3:-}"
  local attempt
  for attempt in $(seq 1 "${STARTUP_TIMEOUT}"); do
    if port_is_open "${port}"; then
      log "[ready] ${name} at 127.0.0.1:${port}"
      return 0
    fi
    if [[ -n "${pid}" ]] && ! kill -0 "${pid}" 2>/dev/null; then
      return 1
    fi
    sleep 1
  done
  return 1
}

start_forward() {
  local service="$1"
  local local_port="$2"
  local remote_port="$3"
  local pid
  if port_is_open "${local_port}"; then
    log "[reuse] 127.0.0.1:${local_port} is already listening; reusing it for ${service}"
    return
  fi
  log "[start] forward service/${service} 127.0.0.1:${local_port} -> ${remote_port}"
  kubectl --kubeconfig "${KUBECONFIG_PATH}" -n "${NAMESPACE}" \
    port-forward --address 127.0.0.1 "service/${service}" "${local_port}:${remote_port}" \
    >"${LOG_DIR}/forward-${service}.log" 2>&1 &
  pid=$!
  started_pid="${pid}"
  owned_pids="${owned_pids} ${pid}"
  process_names="${process_names} forward-${service}:${pid}"
  if ! wait_for_port "${service}" "${local_port}" "${pid}"; then
    tail -n 30 "${LOG_DIR}/forward-${service}.log" >&2 || true
    die "port-forward for service/${service} failed"
  fi
}

start_process() {
  local name="$1"
  local directory="$2"
  shift 2
  local pid
  log "[start] ${name} (log: ${LOG_DIR}/${name}.log)"
  (
    cd "${directory}"
    exec "$@"
  ) >"${LOG_DIR}/${name}.log" 2>&1 &
  pid=$!
  started_pid="${pid}"
  owned_pids="${owned_pids} ${pid}"
  process_names="${process_names} ${name}:${pid}"
}

assert_processes_alive() {
  local entry name pid
  sleep 2
  for entry in ${process_names}; do
    name="${entry%%:*}"
    pid="${entry##*:}"
    if ! kill -0 "${pid}" 2>/dev/null; then
      tail -n 40 "${LOG_DIR}/${name}.log" >&2 2>/dev/null || true
      die "${name} exited during startup"
    fi
  done
}

monitor_processes() {
  local entry name pid
  while true; do
    for entry in ${process_names}; do
      name="${entry%%:*}"
      pid="${entry##*:}"
      if ! kill -0 "${pid}" 2>/dev/null; then
        log "[failed] ${name} exited; last log lines:"
        tail -n 40 "${LOG_DIR}/${name}.log" >&2 2>/dev/null || true
        return 1
      fi
    done
    sleep 2
  done
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-avatar) WITH_AVATAR=1 ;;
    --no-cloud) WITH_CLOUD=0 ;;
    --no-web) WITH_WEB=0 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1 (use --help)" ;;
  esac
  shift
done

require_command kubectl
require_command nc
require_command java
require_command seq
[[ "${STARTUP_TIMEOUT}" =~ ^[1-9][0-9]*$ ]] || die "STARTUP_TIMEOUT must be a positive integer"
if [[ "${WITH_WEB}" -eq 1 ]]; then require_command npm; fi
if [[ "${WITH_AVATAR}" -eq 1 ]]; then require_command uv; fi
[[ -f "${KUBECONFIG_PATH}" ]] || die "kubeconfig not found: ${KUBECONFIG_PATH}"

mkdir -p "${LOG_DIR}"
: >"${LOG_DIR}/startup.log"
trap cleanup EXIT INT TERM

log "Building Java 25 services..."
(cd "${SCRIPT_DIR}" && ./mvnw -B -ntp -DskipTests package)

log "Checking Kubernetes infrastructure in namespace ${NAMESPACE}..."
kubectl --kubeconfig "${KUBECONFIG_PATH}" -n "${NAMESPACE}" get \
  deployment/mysql deployment/redis deployment/onnx-classifier \
  daemonset/cotriage-log-collector >/dev/null
kubectl --kubeconfig "${KUBECONFIG_PATH}" -n "${NAMESPACE}" wait \
  --for=condition=available --timeout=120s \
  deployment/mysql deployment/redis deployment/onnx-classifier

log "Clearing Redis database before startup..."
kubectl --kubeconfig "${KUBECONFIG_PATH}" -n "${NAMESPACE}" \
  exec deployment/redis -- redis-cli FLUSHDB | tee "${LOG_DIR}/redis-cleanup.log"

log "Restarting cluster log-collector..."
kubectl --kubeconfig "${KUBECONFIG_PATH}" -n "${NAMESPACE}" \
  rollout restart daemonset/cotriage-log-collector
kubectl --kubeconfig "${KUBECONFIG_PATH}" -n "${NAMESPACE}" \
  rollout status daemonset/cotriage-log-collector --timeout=180s

start_forward mysql "${MYSQL_LOCAL_PORT}" 3306
start_forward redis "${REDIS_LOCAL_PORT}" 6379
start_forward onnx-classifier "${ONNX_LOCAL_PORT}" 18080

start_process server "${SCRIPT_DIR}/server" env \
  TZ=UTC LISTEN_ADDR=":${SERVER_LOCAL_PORT}" KUBECONFIG="${KUBECONFIG_PATH}" \
  MYSQL_DSN="${MYSQL_DSN}" REDIS_ADDR="127.0.0.1:${REDIS_LOCAL_PORT}" \
  SERVER_MIGRATION_TIMEOUT="${SERVER_MIGRATION_TIMEOUT:-3m}" \
  SERVER_SEED_TIMEOUT="${SERVER_SEED_TIMEOUT:-3m}" \
  java -Xms64m -Xmx256m -jar "${SCRIPT_DIR}/server/target/server-0.1.0-SNAPSHOT.jar"
server_pid="${started_pid}"
if ! wait_for_port server "${SERVER_LOCAL_PORT}" "${server_pid}"; then
  tail -n 40 "${LOG_DIR}/server.log" >&2 || true
  die "server failed to listen on port ${SERVER_LOCAL_PORT}"
fi

start_process edge-infer "${SCRIPT_DIR}/edge-infer" env \
  REDIS_ADDR="127.0.0.1:${REDIS_LOCAL_PORT}" \
  RULES_FILE="${SCRIPT_DIR}/edge-infer/config/rules.example.json" \
  TEMPLATES_FILE="${SCRIPT_DIR}/edge-infer/config/templates.example.json" \
  EDGE_CONFIG_URL="http://127.0.0.1:${SERVER_LOCAL_PORT}/api/v1/edge-config" \
  CLASSIFIER_URL="http://127.0.0.1:${ONNX_LOCAL_PORT}/predict" \
  LLM_URL="${EDGE_LLM_URL:-http://100.85.92.31:18081/v1/chat/completions}" \
  LLM_TIMEOUT="${EDGE_LLM_TIMEOUT:-30s}" \
  CONSUMER_NAME="edge-infer-local" \
  java -Xms64m -Xmx256m -jar "${SCRIPT_DIR}/edge-infer/target/edge-infer-0.1.0-SNAPSHOT.jar"

start_process router-controller "${SCRIPT_DIR}/router-controller" env \
  REDIS_ADDR="127.0.0.1:${REDIS_LOCAL_PORT}" MYSQL_DSN="${MYSQL_DSN}" \
  CONSUMER_NAME="router-controller-local" AUTO_MIGRATE=1 \
  ROUTER_MIGRATION_TIMEOUT="${ROUTER_MIGRATION_TIMEOUT:-3m}" \
  java -Xms64m -Xmx256m -jar "${SCRIPT_DIR}/router-controller/target/router-controller-0.1.0-SNAPSHOT.jar"

if [[ "${WITH_CLOUD}" -eq 1 ]]; then
  start_process cloud-gateway "${SCRIPT_DIR}/cloud-gateway" env \
    REDIS_ADDR="127.0.0.1:${REDIS_LOCAL_PORT}" MYSQL_DSN="${MYSQL_DSN}" \
    SERVER_BASE_URL="http://127.0.0.1:${SERVER_LOCAL_PORT}" \
    CLOUD_GATEWAY_CONSUMER="cloud-gateway-local" \
    java -Xms64m -Xmx256m -jar "${SCRIPT_DIR}/cloud-gateway/target/cloud-gateway-0.1.0-SNAPSHOT.jar"
fi

if [[ "${WITH_WEB}" -eq 1 ]]; then
  [[ -d "${SCRIPT_DIR}/web/node_modules" ]] || die "web/node_modules is missing; run 'cd web && npm install' once"
  if [[ "${WITH_AVATAR}" -eq 1 ]]; then
    start_process web "${SCRIPT_DIR}/web" env \
      VITE_LIVEKIT_TOKEN_ENDPOINT="http://127.0.0.1:${AVATAR_TOKEN_PORT}/token" \
      npm run dev -- --host 127.0.0.1 --port "${WEB_LOCAL_PORT}"
  else
    start_process web "${SCRIPT_DIR}/web" npm run dev -- --host 127.0.0.1 --port "${WEB_LOCAL_PORT}"
  fi
  web_pid="${started_pid}"
  if ! wait_for_port web "${WEB_LOCAL_PORT}" "${web_pid}"; then
    tail -n 40 "${LOG_DIR}/web.log" >&2 || true
    die "Web failed to listen on port ${WEB_LOCAL_PORT}"
  fi
fi

if [[ "${WITH_AVATAR}" -eq 1 ]]; then
  AVATAR_DIR="${SCRIPT_DIR}/platform-integrations/livekit-agents-demo/livekit-agents-reference-demo/backend/cascade"
  [[ -f "${AVATAR_DIR}/.env" ]] || die "avatar configuration is missing: ${AVATAR_DIR}/.env"
  start_process avatar-token-server "${AVATAR_DIR}" env \
    TOKEN_SERVER_PORT="${AVATAR_TOKEN_PORT}" uv run token_server.py
  avatar_token_pid="${started_pid}"
  if ! wait_for_port avatar-token-server "${AVATAR_TOKEN_PORT}" "${avatar_token_pid}"; then
    tail -n 40 "${LOG_DIR}/avatar-token-server.log" >&2 || true
    die "avatar token server failed to listen on port ${AVATAR_TOKEN_PORT}"
  fi
  start_process avatar-agent "${AVATAR_DIR}" env \
    VAD_ACTIVATION_THRESHOLD="${VAD_ACTIVATION_THRESHOLD:-0.55}" \
    uv run agent.py dev
fi

assert_processes_alive
log "CoTriage is ready."
log "Server: http://127.0.0.1:${SERVER_LOCAL_PORT}"
if [[ "${WITH_WEB}" -eq 1 ]]; then log "Web:    http://127.0.0.1:${WEB_LOCAL_PORT}"; fi
if [[ "${WITH_AVATAR}" -eq 1 ]]; then log "Avatar token endpoint: http://127.0.0.1:${AVATAR_TOKEN_PORT}/token"; fi
log "Press Ctrl-C to stop. Follow a component with: tail -f ${LOG_DIR}/<component>.log"

monitor_processes
