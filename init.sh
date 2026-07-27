#!/usr/bin/env bash

set -euo pipefail

LOG_DIR=".dev-logs"
CONSOLE_IMAGE="${CONSOLE_IMAGE:="quay.io/openshift/origin-console:latest"}"
BACKEND_PORT=8080
PLUGIN_PORT=9001
CONSOLE_PORT=9000
TIMEOUT=60
PID_DIR=".dev-pids"

wait_for_port() {
  local port=$1
  local label=$2
  local elapsed=0

  while ! bash -c "echo >/dev/tcp/localhost/$port" 2>/dev/null; do
    if [ $elapsed -ge $TIMEOUT ]; then
      echo "Error: $label did not start within ${TIMEOUT}s. Check $LOG_DIR/ for details."
      exit 1
    fi
    echo "Waiting for $label (port $port)... ${elapsed}s"
    sleep 1
    elapsed=$((elapsed + 1))
  done
}

kill_tree() {
  local pid=$1
  local children
  children=$(pgrep -P "$pid" 2>/dev/null || true)
  for child in $children; do
    kill_tree "$child"
  done
  kill "$pid" 2>/dev/null || true
}

stop_pid() {
  local pidfile="$PID_DIR/$1"
  local label=$2

  if [ ! -f "$pidfile" ]; then
    return
  fi

  local pid
  pid=$(cat "$pidfile")
  if kill -0 "$pid" 2>/dev/null; then
    kill_tree "$pid"
    while kill -0 "$pid" 2>/dev/null; do sleep 0.1; done
    echo "Stopped $label (PID $pid)."
  fi
  rm -f "$pidfile"
}

random_free_port() {
  local port
  while true; do
    port=$((RANDOM % 50001 + 10000))
    if ! bash -c "echo >/dev/tcp/localhost/$port" 2>/dev/null; then
      echo "$port"
      return
    fi
  done
}

write_dev_env() {
  cat > .dev-env.json <<EOF
{
  "backendPort": $BACKEND_PORT,
  "pluginPort": $PLUGIN_PORT,
  "consolePort": $CONSOLE_PORT
}
EOF
}

start_backend() {
  echo "Building dev server..."
  (cd backend && go build -buildvcs=false -o ../bin/devserver ./cmd/devserver)
  echo "Starting dev server..."
  ./bin/devserver --port "$BACKEND_PORT" >>"$LOG_DIR/backend.log" 2>&1 &
  echo $! > "$PID_DIR/backend.pid"
}

stop_backend() {
  stop_pid "backend.pid" "Go backend"
}

stop_plugin() {
  stop_pid "webpack.pid" "plugin dev server"
}

stop_console() {
  local cidfile="$PID_DIR/console.cid"

  if [ ! -f "$cidfile" ]; then
    return
  fi

  local cid
  cid=$(cat "$cidfile")
  if podman stop "$cid" >/dev/null 2>&1; then
    echo "Stopped OpenShift console (container $cid)."
  fi
  rm -f "$cidfile"
}

stop_dev() {
  stop_backend
  stop_plugin
  stop_console
  rm -f .dev-env.json
}

check_prerequisites() {
  if ! command -v oc &>/dev/null; then
    echo "Error: oc CLI not found. Install from https://console.redhat.com/openshift/downloads"
    exit 1
  fi

  if ! oc whoami &>/dev/null; then
    echo "Error: not logged in to OpenShift. Run 'oc login' first."
    exit 1
  fi
}

install_dependencies() {
  if [ ! -d "node_modules" ]; then
    echo "Installing dependencies..."
    yarn install
  fi
}

start_plugin() {
  echo "Starting plugin dev server..."
  PLUGIN_PORT="$PLUGIN_PORT" yarn start >"$LOG_DIR/webpack.log" 2>&1 &
  echo $! > "$PID_DIR/webpack.pid"
}

start_console() {
  echo "Starting OpenShift console..."
  ./start-console.sh \
    --backend-port "$BACKEND_PORT" \
    --plugin-port "$PLUGIN_PORT" \
    --console-port "$CONSOLE_PORT" \
    --cidfile "$PID_DIR/console.cid" \
    >"$LOG_DIR/console.log" 2>&1 &
}

print_status() {
  echo ""
  echo "Dev environment started:"
  echo "  Backend: http://localhost:$BACKEND_PORT"
  echo "  Console: http://localhost:$CONSOLE_PORT"
  echo "  Logs:    $LOG_DIR/"
  echo ""
  echo "To stop: ./init.sh --stop"
}

main() {
  mkdir -p "$LOG_DIR" "$PID_DIR" bin
  check_prerequisites
  install_dependencies
  stop_dev
  write_dev_env
  start_backend
  wait_for_port "$BACKEND_PORT" "Go backend"
  start_plugin
  wait_for_port "$PLUGIN_PORT" "Plugin dev server"
  start_console
  wait_for_port "$CONSOLE_PORT" "OpenShift console"
  print_status
}

case "${1:-}" in
  --stop)
    stop_dev
    ;;
  --randomize-ports)
    BACKEND_PORT=$(random_free_port)
    PLUGIN_PORT=$(random_free_port)
    while [ "$PLUGIN_PORT" -eq "$BACKEND_PORT" ]; do
      PLUGIN_PORT=$(random_free_port)
    done
    CONSOLE_PORT=$(random_free_port)
    while [ "$CONSOLE_PORT" -eq "$BACKEND_PORT" ] || [ "$CONSOLE_PORT" -eq "$PLUGIN_PORT" ]; do
      CONSOLE_PORT=$(random_free_port)
    done
    main
    ;;
  "")
    main
    ;;
  *)
    echo "Usage: $0 [--stop | --randomize-ports]"
    exit 1
    ;;
esac
