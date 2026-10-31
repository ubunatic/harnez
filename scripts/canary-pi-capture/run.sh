#!/usr/bin/env bash
set -euo pipefail

# Issue 1021: real-Pi capture canary. No external model is reachable:
# everything runs in a loopback-only network namespace against a local stub, with a temporary HOME.
# The PID namespace ends every process with the run. Global Pi is only executed, never changed.
# Usage: scripts/canary-pi-capture/run.sh [--log FILE]
# PI_BIN selects the Pi executable (default: pi on PATH).

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd "$script_dir/../.." && pwd)"

if test "${1:-}" = "--inside"
then shift
     ip link set lo up
     exec node "$script_dir/driver.mjs" "$1" "$2" "$repo_dir/.pi/extensions/background-tasks.ts"
fi

log_file=""
if test "${1:-}" = "--log"
then log_file="$2"
     shift 2
fi
pi_bin="$(readlink -f "$(command -v "${PI_BIN:-pi}")")"
"$pi_bin" --version

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/pi-capture-canary.XXXXXX")"
trap 'rm -rf "$work_dir"' EXIT

if test -n "$log_file"
then timeout 90s unshare -Urnpf --kill-child "$0" --inside "$work_dir" "$pi_bin" 2>&1 | tee "$log_file"
else timeout 90s unshare -Urnpf --kill-child "$0" --inside "$work_dir" "$pi_bin" 2>&1
fi
