#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)
bin=${HARNEZ_BIN:-harnez}
payload="$repo_dir/cmd/harnez/testdata/statusline-agy.json"
run_count=${STATUSLINE_BENCH_RUNS:-200}
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

if ! command -v "$bin" >/dev/null 2>&1 && ! test -x "$bin"
then
  printf 'ERROR: harnez binary not found: %s\n' "$bin" >&2
  exit 1
fi

mkdir -p "$tmp_dir/home" "$tmp_dir/data"
export HOME="$tmp_dir/home"
export XDG_DATA_HOME="$tmp_dir/data"
samples="$tmp_dir/samples"
sorted="$tmp_dir/sorted"
: > "$samples"

for ((i = 0; i < run_count; i++))
do
  started=$(date +%s%N)
  "$bin" statusline --agent agy < "$payload" >/dev/null
  finished=$(date +%s%N)
  printf '%s\n' "$(((finished - started) / 1000000))" >> "$samples"
done

sort -n "$samples" > "$sorted"
awk -v count="$run_count" '
  NR == int((count + 1) / 2) { p50 = $1 }
  NR == int(count * 0.99 + 0.999999) { p99 = $1 }
  NR == count { max = $1 }
  END { printf "calls=%d p50=%dms p99=%dms max=%dms\n", count, p50, p99, max }
' "$sorted"
