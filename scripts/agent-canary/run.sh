#!/usr/bin/env bash
# Build Containerfile.app and Containerfile.lmcoder and run the agent smoke
# test in one podman pod, so both share loopback (issue 723).
# Run from the project root: make smoke-container
#
# Environment:
#   LMCODER_MODEL      lmcoder model name or alias (default: canary, ~490 MB)
#   LMCODER_MODEL_DIR  host directory with already downloaded models, mounted
#                      read-only instead of downloading into the cache volume
#   LMCODER_CACHE_DIR  host directory for downloaded models instead of the
#                      cache volume, e.g. on a scratch disk

set -euo pipefail

pod=harnez-smoke
app_image=harnez-app
lmcoder_image=harnez-lmcoder
cache_volume=harnez-lmcoder-cache
model="${LMCODER_MODEL:-canary}"
model_dir="${LMCODER_MODEL_DIR:-}"
cache_dir="${LMCODER_CACHE_DIR:-}"

detect_container_cmd() {
   if test -n "${CONTAINER_CMD:-}"
   then printf '%s' "${CONTAINER_CMD}"
        return 0
   fi
   if command -v podman >/dev/null 2>&1
   then printf 'podman'
   elif command -v docker >/dev/null 2>&1
   then printf 'docker'
   else printf '%s\n' "ERROR: neither podman nor docker is installed" >&2
        exit 1
   fi
}

container_cmd=$(detect_container_cmd)

cleanup() {
   "$container_cmd" pod rm -f "${pod}" >/dev/null 2>&1 || true
}

"$container_cmd" build -t "${app_image}" -f Containerfile.app .
"$container_cmd" build -t "${lmcoder_image}" -f Containerfile.lmcoder .

cleanup
trap cleanup EXIT
"$container_cmd" pod create --name "${pod}" >/dev/null

lmcoder_args=(-d --pod "${pod}" --name "${pod}-lmcoder" -e "LMCODER_MODEL=${model}")
if test -n "${cache_dir}"
then mkdir -p "${cache_dir}"
     lmcoder_args+=(-v "${cache_dir}:/cache:z")
else "$container_cmd" volume exists "${cache_volume}" || "$container_cmd" volume create "${cache_volume}" >/dev/null
     lmcoder_args+=(-v "${cache_volume}:/cache")
fi
if test -n "${model_dir}"
then lmcoder_args+=(-v "${model_dir}:/cache/models:ro,z")
fi
"$container_cmd" run "${lmcoder_args[@]}" "${lmcoder_image}" >/dev/null

if ! "$container_cmd" run --rm --pod "${pod}" --cap-drop=all --security-opt no-new-privileges "${app_image}" smoke-test
then printf '\n%s\n' "--- lmcoder log (last 30 lines)" >&2
     "$container_cmd" logs --tail 30 "${pod}-lmcoder" >&2
     exit 1
fi
