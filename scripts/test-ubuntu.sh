#!/usr/bin/env bash
# Run the test suite inside an Ubuntu 24.04 container (with default Git 2.43).
#
# Usage: scripts/test-ubuntu.sh
#
# Env:
#   IMAGE=ubuntu:24.04
#   CONTAINER_CMD=podman # or docker

set -euo pipefail

image="${IMAGE:-ubuntu:24.04}"

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

printf 'Running test suite in %s using %s...\n' "$image" "$container_cmd"

script=$(cat <<'EOF'
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq --no-install-recommends curl ca-certificates git >/dev/null

if ! command -v go >/dev/null 2>&1; then
   curl -fsSL https://go.dev/dl/go1.24.0.linux-amd64.tar.gz | tar -C /usr/local -xz
   export PATH="/usr/local/go/bin:$PATH"
fi

printf 'Git version: %s\n' "$(git --version)"
printf 'Go version:  %s\n' "$(go version)"

# Copy source to a writable workdir inside the container so tests can run cleanly
mkdir -p /work
cp -R /src/. /work/
cd /work

GOWORK=off go test ./...
EOF
)

"$container_cmd" run --rm \
   -v "$(pwd):/src:ro" \
   -e DEBIAN_FRONTEND=noninteractive \
   "$image" \
   bash -c "$script"
