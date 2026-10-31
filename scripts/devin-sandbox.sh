#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="agent-sandbox"
CONTAINER_NAME="agent-sandbox-run"
WORKSPACE_DIR="${PWD}"
LOG_DIR="${PWD}/.sandbox-logs"

mkdir -p "$LOG_DIR"

EXTRA_ARGS=()

MITM_CERT="${HOME}/.mitmproxy/mitmproxy-ca-cert.pem"
if test -f "$MITM_CERT"
then EXTRA_ARGS+=(
       --add-host=host.containers.internal:host-gateway
       -e HTTP_PROXY="http://host.containers.internal:8080"
       -e HTTPS_PROXY="http://host.containers.internal:8080"
       -e SSL_CERT_FILE="/mitm-ca.pem"
       -e NODE_EXTRA_CA_CERTS="/mitm-ca.pem"
       -v "$MITM_CERT:/mitm-ca.pem:ro"
     )
fi

printf 'Starting sandbox container (%s)...\n' "$IMAGE_NAME"
printf 'Logs recorded in: %s\n' "$LOG_DIR"

exec podman run --rm -it \
  --name "$CONTAINER_NAME" \
  --userns=keep-id \
  -v "${WORKSPACE_DIR}:/workspace:rw" \
  -w /workspace \
  "${EXTRA_ARGS[@]}" \
  "$IMAGE_NAME" \
  "$@"
