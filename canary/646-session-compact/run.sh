#!/usr/bin/env bash
set -euo pipefail

plugin_dir=$(cd "$(dirname "$0")" && pwd)
run_dir=$(mktemp -d "${TMPDIR:-/tmp}/harnez-646-XXXXXX")
server_pid=
cleanup() {
  status=$?
  if test -n "$server_pid"
  then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  if test "$status" -eq 0
  then
    rm -rf "$run_dir"
  else
    printf 'Probe artifacts retained: %s\n' "$run_dir" >&2
  fi
  return "$status"
}
trap cleanup EXIT

python3 -m http.server 18764 --bind 127.0.0.1 --directory "$run_dir" >"$run_dir/http.log" 2>&1 &
server_pid=$!

for route in a child
do
  probe_dir="$run_dir/probe-$route"
  mkdir -p "$probe_dir/hooks" "$probe_dir/.claude-plugin"
  cp "$plugin_dir/.claude-plugin/plugin.json" "$probe_dir/.claude-plugin/plugin.json"
  cp "$plugin_dir/hooks/hooks.json" "$probe_dir/hooks/hooks.json"
  cp "$plugin_dir/hooks/probe.ts" "$probe_dir/hooks/probe.ts"
  if test "$route" = child
  then
    cp "$plugin_dir/hooks/probe-child.ts" "$probe_dir/hooks/probe.ts"
  fi
done

run_probe() {
  label=$1
  probe_dir=$2
  report_name=$3
  debug_file="$run_dir/$label-debug.log"
  CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1 claude -p --model haiku \
    --plugin-dir "$probe_dir" --output-format json --debug-file "$debug_file" \
    'Reply with READY only.' > "$label-first.json"
  session_id=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["session_id"])' "$label-first.json")
  CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1 claude -p --model haiku \
    --plugin-dir "$probe_dir" --output-format json --debug-file "$debug_file" --resume "$session_id" \
    '/compact' > "$label-compact.json"

  python3 - "$label" <<'PY'
import json
import sys

label = sys.argv[1]
compact = json.load(open(f"{label}-compact.json", encoding="utf-8"))
print(f"{label} compact command: {compact.get('local_command')}")
if compact.get("local_command") != "compact":
    raise SystemExit(f"FAIL: {label} manual compaction was not observed")
PY

  printf '%s debug log: %s\n' "$label" "$debug_file"
  if ! rg -Fq 'session.compact settled' "$debug_file"
  then
    printf 'FAIL: %s debug log does not show the compact hook settling.\n' "$label" >&2
    exit 1
  fi
  if ! rg -Fq "session.compact bridge $report_name" "$debug_file"
  then
    printf 'FAIL: %s route report is absent from the debug log.\n' "$label" >&2
    exit 1
  fi
  rg -F "session.compact bridge $report_name" "$debug_file"
}

cd "$run_dir"
printf 'Scratch cwd: %s\n' "$run_dir"
run_probe a "$run_dir/probe-a" A
run_probe child "$run_dir/probe-child" B
