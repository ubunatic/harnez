#!/usr/bin/env bash
set -euo pipefail

# Issue 1047: real-Pi canary for task completions during compaction. No external model is reachable:
# everything runs in a loopback-only network namespace against a local stub, with a temporary HOME.
# The PID namespace ends every process with the run. Global Pi is only executed, never changed.
# Usage: scripts/canary-pi-compaction/run.sh [--log FILE] [comma-separated scenario names]
# PI_BIN selects the Pi executable (default: pi on PATH).

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd "$script_dir/../.." && pwd)"

if test "${1:-}" = "--inside"
then shift
     ip link set lo up
     exec node "$script_dir/driver.mjs" "$1" "$2" "$repo_dir/.pi/extensions/background-tasks.ts" "$script_dir/fixture.ts" "${3:-}"
fi

log_file=""
if test "${1:-}" = "--log"
then log_file="$2"
     shift 2
fi
pi_bin="$(readlink -f "$(command -v "${PI_BIN:-pi}")")"
"$pi_bin" --version

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/pi-compaction-canary.XXXXXX")"
trap 'rm -rf "$work_dir"' EXIT

if test -n "$log_file"
then unshare -Urnpf --kill-child "$0" --inside "$work_dir" "$pi_bin" "${1:-}" 2>&1 | tee "$log_file"
else unshare -Urnpf --kill-child "$0" --inside "$work_dir" "$pi_bin" "${1:-}" 2>&1
fi
