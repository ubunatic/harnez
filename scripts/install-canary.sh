#!/usr/bin/env bash
# Canary: install the `latest` harnez release from Codeberg into a clean
# container (no Go toolchain, no dev tools) and verify `harnez --version`
# reports the tag that was just fetched.
#
# Probes the real external mechanism (Codeberg releases API + the tar.gz
# archive artifact) rather than mocking it. See docs/other/Canary.md.
#
# Usage: scripts/install-canary.sh
#
# Env:
#   OWNER=ubunatic
#   REPO=harnez
#   ARCH=x86_64   # or aarch64
#   IMAGE=debian:bookworm-slim

set -euo pipefail

owner="${OWNER:-ubunatic}"
repo="${REPO:-harnez}"
arch="${ARCH:-x86_64}"
image="${IMAGE:-debian:bookworm-slim}"

require_podman() {
   if ! command -v podman >/dev/null 2>&1
   then printf '%s\n' "ERROR: podman is required" >&2
        exit 1
   fi
}

require_podman

api_url="https://codeberg.org/api/v1/repos/${owner}/${repo}/releases/latest"
tag=$(curl -fsSL "$api_url" | grep -o '"tag_name":"[^"]*"' | head -1 | cut -d'"' -f4)

if test -z "$tag"
then printf 'ERROR: could not determine latest tag from %s\n' "$api_url" >&2
     exit 1
fi

version="${tag#v}"
asset="${repo}-${version}-${arch}-linux.tar.gz"
download_url="https://codeberg.org/${owner}/${repo}/releases/download/${tag}/${asset}"

printf 'Verifying %s %s (%s) installs and reports its version...\n' "$repo" "$tag" "$arch"

script=$(cat <<EOF
set -euo pipefail
apt-get update -qq
apt-get install -y -qq --no-install-recommends curl ca-certificates >/dev/null
cd /tmp
curl -fsSL -o release.tar.gz "$download_url"
tar -xzf release.tar.gz
got=\$(./${repo} --version)
printf 'reported: %s\n' "\$got"
case "\$got" in
  *"$version"*) printf 'PASS: %s --version reports %s\n' "$repo" "$version" ;;
  *) printf 'FAIL: expected version %s, got: %s\n' "$version" "\$got" >&2; exit 1 ;;
esac
EOF
)

podman run --rm \
   --read-only=false \
   -e DEBIAN_FRONTEND=noninteractive \
   "$image" \
   bash -c "$script"
