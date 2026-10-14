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

# Install the toolchain go.mod requires; GOTOOLCHAIN=auto would otherwise
# download it again on every run.
go_version=$(awk '$1 == "go" { print $2; exit }' go.mod)

script=$(cat <<'EOF'
set -euo pipefail
apt-get update -qq
apt-get install -y -qq --no-install-recommends curl ca-certificates git >/dev/null

case "$(uname -m)" in
   x86_64)  go_arch=amd64 ;;
   aarch64) go_arch=arm64 ;;
   *) printf 'ERROR: unsupported arch %s\n' "$(uname -m)" >&2
      exit 1 ;;
esac
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" |
   tar -C /usr/local -xz
export PATH="/usr/local/go/bin:$PATH"

printf 'Git version: %s\n' "$(git --version)"
printf 'Go version:  %s\n' "$(go version)"

# Copy the read-only source mount to a writable workdir.
mkdir -p /work
cp -R /src/. /work/
GOWORK=off go -C /work test ./...
EOF
)

"$container_cmd" run --rm \
   -v "$(pwd):/src:ro" \
   -e DEBIAN_FRONTEND=noninteractive \
   -e GO_VERSION="$go_version" \
   "$image" \
   bash -c "$script"
