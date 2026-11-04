#!/usr/bin/env bash
# scripts/jules-setup.sh — one-shot environment setup for cloud agents (Jules, Codex, Claude web)
# on Ubuntu/Debian. Paste it into the Jules environment setup, or run it from the repo root.
# Installs what Containerfile.test installs (the CI source of truth): apt tools, the Go version
# from go.mod, Node 22 for make test-pi; then downloads modules and installs harnez to ~/go/bin.
# Idempotent: tools already at the right version are kept.
set -euo pipefail

NODE_VERSION="${NODE_VERSION:-22.23.1}"   # keep in sync with Containerfile.test

repo=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo"

sudo_cmd=""
if test "$(id -u)" -ne 0
then sudo_cmd="sudo"
fi

case "$(uname -m)" in
x86_64)  go_arch=amd64 node_arch=x64 ;;
aarch64) go_arch=arm64 node_arch=arm64 ;;
*)       printf 'ERROR: unsupported arch %s\n' "$(uname -m)" >&2
         exit 1 ;;
esac

printf '==> apt packages\n'
export DEBIAN_FRONTEND=noninteractive
$sudo_cmd apt-get update -y -qq
# gcc/libc6-dev: go test -race; minisign: internal/release; python3: agent stop fixture;
# procps: process tests; git-lfs: docs/Containerfile.md.
$sudo_cmd apt-get install -y -qq --no-install-recommends \
   ca-certificates curl gcc git git-lfs libc6-dev make minisign procps python3 ripgrep
git lfs install >/dev/null 2>&1 || true

printf '==> Go\n'
go_version=$(awk '$1 == "go" { print $2; exit }' go.mod)
if ! command -v go >/dev/null || test "$(go env GOVERSION 2>/dev/null)" != "go${go_version}"
then curl -fsSL "https://go.dev/dl/go${go_version}.linux-${go_arch}.tar.gz" -o /tmp/go.tgz
     $sudo_cmd rm -rf /usr/local/go
     $sudo_cmd tar -C /usr/local -xzf /tmp/go.tgz
     rm -f /tmp/go.tgz
fi
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"
go version

printf '==> Node\n'
if ! command -v node >/dev/null || test "$(node --version)" != "v${NODE_VERSION}"
then curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${node_arch}.tar.gz" \
        | $sudo_cmd tar -C /usr/local --strip-components=1 -xz
fi
node --version

printf '==> Go modules and harnez\n'
GOWORK=off go mod download
GOWORK=off make install

# Later agent shells are often non-interactive and skip ~/.bashrc, so link into /usr/local/bin.
printf '==> link go, gofmt, harnez into /usr/local/bin\n'
for bin in /usr/local/go/bin/go /usr/local/go/bin/gofmt "$HOME/go/bin/harnez"
do $sudo_cmd ln -sf "$bin" /usr/local/bin/
done
harnez --version

printf '==> done: run make lint, make test-short or make test\n'
