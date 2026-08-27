#!/usr/bin/env bash
# Build and run the shared harnez Pi/OpenCode agent-canary container.
# Run from the project root: scripts/agent-canary/run.sh static

set -euo pipefail

image="${HARNEZ_AGENT_CANARY_IMAGE:-harnez-agent-canary}"
proxy_port="${HARNEZ_AGENT_CANARY_PROXY_PORT:-8736}"
backend_port="${HARNEZ_AGENT_CANARY_BACKEND_PORT:-8737}"
model="${HARNEZ_AGENT_CANARY_MODEL:-qwen2.5-0.5b-instruct-q4}"

usage() {
   printf '%s\n' "Usage: scripts/agent-canary/run.sh <build|static|pi|opencode>"
   printf '%s\n' ""
   printf '%s\n' "Environment:"
   printf '%s\n' "  HARNEZ_AGENT_CANARY_IMAGE=${image}"
   printf '%s\n' "  HARNEZ_AGENT_CANARY_PROXY_PORT=${proxy_port}"
   printf '%s\n' "  HARNEZ_AGENT_CANARY_BACKEND_PORT=${backend_port}"
   printf '%s\n' "  HARNEZ_AGENT_CANARY_MODEL=${model}"
   printf '%s\n' ""
   printf '%s\n' "Live canary prerequisite, using local lmcoder only:"
   printf '%s\n' "  lmcoder start --model ${model} --port ${backend_port} --log-file /tmp/harnez-agent-canary-lmcoder.log"
   printf '%s\n' "  lmcoder proxy --insecure --backend-host 127.0.0.1 --backend-port ${backend_port} --listen-port ${proxy_port}"
   printf '%s\n' ""
   printf '%s\n' "Do not use the default 8735 proxy or any um760-backed proxy for this canary."
}

require_podman() {
   if ! command -v podman >/dev/null 2>&1
   then printf '%s\n' "ERROR: podman is required" >&2
        exit 1
   fi
}

build_image() {
   require_podman
   podman build -t "${image}" -f scripts/agent-canary/Containerfile .
}

container_common_args() {
   printf '%s\n' \
      "--rm" \
      "--read-only" \
      "--cap-drop=all" \
      "--security-opt" \
      "no-new-privileges" \
      "--userns=keep-id" \
      "--tmpfs" \
      "/tmp" \
      "-e" \
      "HOME=/tmp/harnez-home" \
      "-e" \
      "XDG_CONFIG_HOME=/tmp/harnez-home/.config" \
      "-e" \
      "XDG_DATA_HOME=/tmp/harnez-home/.local/share" \
      "-e" \
      "XDG_CACHE_HOME=/tmp/harnez-home/.cache" \
      "-e" \
      "XDG_STATE_HOME=/tmp/harnez-home/.local/state" \
      "-e" \
      "PI_CODING_AGENT_DIR=/tmp/harnez-home/.pi/agent" \
      "-e" \
      "PI_CODING_AGENT_SESSION_DIR=/tmp/harnez-home/.pi/sessions" \
      "-e" \
      "HARNEZ_AGENT_CANARY_PROXY_PORT=${proxy_port}" \
      "-v" \
      "${PWD}:/work:ro,Z" \
      "-w" \
      "/work"
}

run_static() {
   require_podman
   mapfile -t args < <(container_common_args)
   podman run "${args[@]}" --entrypoint harnez-agent-canary-check "${image}"
}

run_pi() {
   require_podman
   mapfile -t args < <(container_common_args)
   podman run "${args[@]}" --entrypoint pi-launch "${image}" \
      --provider llamacpp \
      --model local-model \
      -p "Reply with exactly one word: PONG"
}

run_opencode() {
   require_podman
   mapfile -t args < <(container_common_args)
   podman run "${args[@]}" --entrypoint opencode-launch "${image}" \
      run \
      --model llamacpp/local-model \
      "Reply with exactly one word: PONG"
}

cmd="${1:-}"
case "${cmd}" in
   build)
      build_image
      ;;
   static)
      build_image
      run_static
      ;;
   pi)
      run_pi
      ;;
   opencode)
      run_opencode
      ;;
   -h|--help|help|"")
      usage
      ;;
   *)
      usage >&2
      exit 2
      ;;
esac
