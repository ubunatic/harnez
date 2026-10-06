#!/usr/bin/env bash
# Build Containerfile.app and Containerfile.lmcoder and run the agent smoke
# test in one podman pod, so both share loopback (issue 723).
# Run from the project root: make smoke-container
#
# Environment:
#   LMCODER_MODEL      lmcoder model name or alias (default: canary, ~490 MB)
#   LMCODER_MODEL_DIR  host directory with already downloaded models, mounted
#                      read-only instead of downloading into the cache volume

set -euo pipefail

pod=harnez-smoke
app_image=harnez-app
lmcoder_image=harnez-lmcoder
cache_volume=harnez-lmcoder-cache
model="${LMCODER_MODEL:-canary}"
model_dir="${LMCODER_MODEL_DIR:-}"

if ! command -v podman >/dev/null 2>&1
then printf '%s\n' "ERROR: podman is required" >&2
     exit 1
fi

cleanup() {
   podman pod rm -f "${pod}" >/dev/null 2>&1 || true
}

podman build -t "${app_image}" -f Containerfile.app .
podman build -t "${lmcoder_image}" -f Containerfile.lmcoder .

cleanup
trap cleanup EXIT
podman volume exists "${cache_volume}" || podman volume create "${cache_volume}" >/dev/null
podman pod create --name "${pod}" >/dev/null

lmcoder_args=(-d --pod "${pod}" --name "${pod}-lmcoder" -e "LMCODER_MODEL=${model}" -v "${cache_volume}:/cache")
if test -n "${model_dir}"
then lmcoder_args+=(-v "${model_dir}:/cache/models:ro,z")
fi
podman run "${lmcoder_args[@]}" "${lmcoder_image}" >/dev/null

if ! podman run --rm --pod "${pod}" --cap-drop=all --security-opt no-new-privileges "${app_image}" smoke-test
then printf '\n%s\n' "--- lmcoder log (last 30 lines)" >&2
     podman logs --tail 30 "${pod}-lmcoder" >&2
     exit 1
fi
