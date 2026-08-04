#!/usr/bin/env bash
# Shared CRC DNS helper container lifecycle functions.
# Source this file: source "$(dirname "${BASH_SOURCE[0]}")/lib/crc-dns.sh"
# Callers are responsible for calling crc_dns::stop on cleanup (e.g. via EXIT trap).

CRC_DNS_IMAGE="${CRC_DNS_IMAGE:-quay.io/mvasek/crc-dns-hack@sha256:ef285298d4b642c149336bb44fa15db36be3dd1e848aa164fdffee29e1df9b99}"
_CRC_DNS_CONTAINER_NAME="crc-dns-$$"

# Start the DNS helper container.
crc_dns::start() {
  local cid
  cid=$(podman run -d --rm \
    --name "$_CRC_DNS_CONTAINER_NAME" \
    --add-host crc-host:host-gateway \
    "$CRC_DNS_IMAGE")

  echo "$cid"
}

crc_dns::ip() {
  local ip
  ip=$(podman inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' \
    "$_CRC_DNS_CONTAINER_NAME")
  if [[ -z "$ip" ]]; then
    echo "crc_dns::ip: sidecar has no IP (wrong network mode?)" >&2
    return 1
  fi
  echo "$ip"
}

crc_dns::stop() {
  podman stop "$_CRC_DNS_CONTAINER_NAME" >/dev/null 2>&1 || true
}
