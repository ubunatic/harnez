#!/usr/bin/env bash
# scripts/jules-setup.sh — one-shot environment setup for cloud agents (Jules, Codex, Claude web)
# on Ubuntu/Debian. Works from any of our repos (harnez, loom, lmcoder, gokay, ...): paste it into
# the Jules environment setup of each repo, or run it from a repo root.
# Installs apt tools (as Containerfile.test, the CI source of truth), Go (the repo's go.mod
# version, else the latest), then harnez, loom, lmcoder and gokay via go install @latest, linked
# into /usr/local/bin, and finally the repo's own `make install` if it has one.
# Idempotent: tools already at the right version are kept.
set -euo pipefail

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
# env, not export: sudo drops DEBIAN_FRONTEND from the caller's environment.
apt_env="env DEBIAN_FRONTEND=noninteractive"
$sudo_cmd $apt_env apt-get update -y -qq
# gcc/libc6-dev: go test -race; minisign: harnez release; python3: agent stop fixture;
# procps: process tests; git-lfs: docs/Containerfile.md.
$sudo_cmd $apt_env apt-get install -y -qq --no-install-recommends \
   ca-certificates curl gcc git git-lfs libc6-dev make minisign procps python3 ripgrep
git lfs install >/dev/null 2>&1 || true

printf '==> Go\n'
go_version=""
if test -f go.mod
then go_version=$(awk '$1 == "go" { print $2; exit }' go.mod)
fi
if test -z "$go_version"
then go_version=$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -1)
     go_version=${go_version#go}
fi
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"
if ! command -v go >/dev/null || test "$(go env GOVERSION 2>/dev/null)" != "go${go_version}"
then curl -fsSL "https://go.dev/dl/go${go_version}.linux-${go_arch}.tar.gz" -o /tmp/go.tgz
     $sudo_cmd rm -rf /usr/local/go
     $sudo_cmd tar -C /usr/local -xzf /tmp/go.tgz
     rm -f /tmp/go.tgz
fi
go version
# A tool whose go.mod needs a newer Go than the repo's fetches that toolchain itself.
export GOTOOLCHAIN=auto GOWORK=off

# harnez `make test-pi` needs Node 22; any preinstalled Node 22 (Jules has one via nvm) is kept.
if test -f go.mod && grep -q '^module ubunatic.com/harnez$' go.mod
then printf '==> Node\n'
     case "$(node --version 2>/dev/null)" in
     v22.*) ;;
     *)     node_version=22.23.1   # keep in sync with Containerfile.test
            curl -fsSL "https://nodejs.org/dist/v${node_version}/node-v${node_version}-linux-${node_arch}.tar.gz" \
               | $sudo_cmd tar -C /usr/local --strip-components=1 -xz ;;
     esac
     node --version
fi

printf '==> harnez, loom, lmcoder, gokay\n'
go install ubunatic.com/harnez/cmd/harnez@latest
go install ubunatic.com/loom/cmd/loom@latest
go install ubunatic.com/lmcoder/cmd/lmcoder@latest
go install ubunatic.com/gokay@latest

# Later agent shells are often non-interactive and skip ~/.bashrc, so link into /usr/local/bin.
printf '==> link go, gofmt and the tools into /usr/local/bin\n'
for bin in /usr/local/go/bin/go /usr/local/go/bin/gofmt "$HOME/go/bin/harnez" "$HOME/go/bin/loom" "$HOME/go/bin/lmcoder" "$HOME/go/bin/gokay"
do $sudo_cmd ln -sf "$bin" /usr/local/bin/
done
harnez --version

if test -f go.mod
then printf '==> Go modules of %s\n' "$repo"
     go mod download
fi
if test -f Makefile && grep -q '^install:' Makefile
then printf '==> make install in %s\n' "$repo"
     make install
fi

printf '==> done\n'
