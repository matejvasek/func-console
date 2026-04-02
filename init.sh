#!/usr/bin/env bash

set -euo pipefail

LOG_DIR=".dev-logs"
CONSOLE_IMAGE="${CONSOLE_IMAGE:="quay.io/openshift/origin-console:latest"}"
BACKEND_PORT=8080
PLUGIN_PORT=9001
CONSOLE_PORT=9000
TIMEOUT=60

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

start_backend() {
  echo "Building Go backend..."
  (cd backend && go build -buildvcs=false -o ../bin/backend .)
  echo "Starting Go backend..."
  ./bin/backend --http-port "$BACKEND_PORT" >"$LOG_DIR/backend.log" 2>&1 &
}

stop_backend() {
  if pgrep -f "bin/backend" >/dev/null 2>&1; then
    pkill -f "bin/backend" && echo "Stopped Go backend."
  fi
}

stop_plugin() {
  if pgrep -f "webpack serve" >/dev/null 2>&1; then
    pkill -f "webpack serve" && echo "Stopped plugin dev server."
  fi
}

stop_console() {
  local container
  container=$(podman ps -q --filter ancestor="$CONSOLE_IMAGE" 2>/dev/null || true)
  if [ -n "$container" ]; then
    podman stop "$container" >/dev/null && echo "Stopped OpenShift console."
  fi
}

stop_dev() {
  stop_backend
  stop_plugin
  stop_console
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
  yarn start >"$LOG_DIR/webpack.log" 2>&1 &
}

start_console() {
  echo "Starting OpenShift console..."
  yarn start-console >"$LOG_DIR/console.log" 2>&1 &
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
  mkdir -p "$LOG_DIR" bin
  check_prerequisites
  install_dependencies
  stop_dev
  start_backend
  wait_for_port "$BACKEND_PORT" "Go backend"
  start_plugin
  wait_for_port "$PLUGIN_PORT" "Plugin dev server"
  start_console
  wait_for_port "$CONSOLE_PORT" "OpenShift console"
  print_status
}

if [ "${1:-}" = "--stop" ]; then
  stop_dev
else
  main
fi
