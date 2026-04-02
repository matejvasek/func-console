#!/usr/bin/env bash

set -euo pipefail

CONSOLE_IMAGE=${CONSOLE_IMAGE:="quay.io/openshift/origin-console:latest"}
CONSOLE_PORT=${CONSOLE_PORT:=9000}
CONSOLE_IMAGE_PLATFORM=${CONSOLE_IMAGE_PLATFORM:="linux/amd64"}

# Plugin metadata is declared in package.json
PLUGIN_NAME="console-functions-plugin"

echo "Starting local OpenShift console..."

set -a
BRIDGE_USER_AUTH="disabled"
BRIDGE_K8S_MODE="off-cluster"
BRIDGE_K8S_AUTH="bearer-token"
BRIDGE_K8S_MODE_OFF_CLUSTER_SKIP_VERIFY_TLS=true
BRIDGE_K8S_MODE_OFF_CLUSTER_ENDPOINT=$(oc whoami --show-server)
# The monitoring operator is not always installed (e.g. for local OpenShift). Tolerate missing config maps.
set +e
BRIDGE_K8S_MODE_OFF_CLUSTER_THANOS=$(oc -n openshift-config-managed get configmap monitoring-shared-config -o jsonpath='{.data.thanosPublicURL}' 2>/dev/null)
BRIDGE_K8S_MODE_OFF_CLUSTER_ALERTMANAGER=$(oc -n openshift-config-managed get configmap monitoring-shared-config -o jsonpath='{.data.alertmanagerPublicURL}' 2>/dev/null)
set -e
BRIDGE_K8S_AUTH_BEARER_TOKEN=$(oc whoami --show-token 2>/dev/null)
BRIDGE_USER_SETTINGS_LOCATION="localstorage"
BRIDGE_I18N_NAMESPACES="plugin__${PLUGIN_NAME}"

# Don't fail if the cluster doesn't have gitops.
set +e
GITOPS_HOSTNAME=$(oc -n openshift-gitops get route cluster -o jsonpath='{.spec.host}' 2>/dev/null)
set -e
if [ -n "$GITOPS_HOSTNAME" ]; then
    BRIDGE_K8S_MODE_OFF_CLUSTER_GITOPS="https://$GITOPS_HOSTNAME"
fi

echo "API Server: $BRIDGE_K8S_MODE_OFF_CLUSTER_ENDPOINT"
echo "Console Image: $CONSOLE_IMAGE"
echo "Console URL: http://localhost:${CONSOLE_PORT}"
echo "Console Platform: $CONSOLE_IMAGE_PLATFORM"

# Prefer podman if installed. Otherwise, fall back to docker.
if [ -x "$(command -v podman)" ]; then
    CONTAINER_CMD="podman"
    if [ "$(uname -s)" = "Linux" ]; then
        # Use host networking on Linux since host.containers.internal is unreachable in some environments.
        PLUGIN_HOST="localhost"
        CONTAINER_NETWORK_OPTS="--network=host"
    else
        PLUGIN_HOST="host.containers.internal"
        CONTAINER_NETWORK_OPTS="-p ${CONSOLE_PORT}:9000"
    fi
else
    CONTAINER_CMD="docker"
    PLUGIN_HOST="host.docker.internal"
    CONTAINER_NETWORK_OPTS="-p ${CONSOLE_PORT}:9000"
fi

BRIDGE_PLUGINS="${PLUGIN_NAME}=http://${PLUGIN_HOST}:9001"
BRIDGE_PLUGIN_PROXY='{"services":[{"consoleAPIPath":"/api/proxy/plugin/'"${PLUGIN_NAME}"'/backend/","endpoint":"http://'"${PLUGIN_HOST}"':8080","authorize":false}]}'

echo "BRIDGE_PLUGINS=$BRIDGE_PLUGINS"
echo "BRIDGE_PLUGIN_PROXY=$BRIDGE_PLUGIN_PROXY"

$CONTAINER_CMD run --pull always --platform $CONSOLE_IMAGE_PLATFORM --rm $CONTAINER_NETWORK_OPTS --env-file <(env | grep ^BRIDGE) $CONSOLE_IMAGE
