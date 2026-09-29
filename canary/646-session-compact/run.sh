#!/usr/bin/env bash
set -euo pipefail

plugin_dir=$(cd "$(dirname "$0")" && pwd)
run_dir=$(mktemp -d "${TMPDIR:-/tmp}/harnez-646-XXXXXX")
server_pid=
cleanup() {
  if test -n "$server_pid"
  then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$run_dir"
}
trap cleanup EXIT

python3 -m http.server 18764 --bind 127.0.0.1 --directory "$run_dir" >"$run_dir/http.log" 2>&1 &
server_pid=$!

cd "$run_dir"
printf 'Scratch cwd: %s\n' "$run_dir"

CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1 claude -p --model haiku \
  --plugin-dir "$plugin_dir" --output-format json \
  'Reply with READY only.' > first.json
session_id=$(python3 -c 'import json; print(json.load(open("first.json"))["session_id"])')
printf 'Session id: %s\n' "$session_id"

CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1 claude -p --model haiku \
  --plugin-dir "$plugin_dir" --output-format json --resume "$session_id" \
  '/compact' > compact.json
cat compact.json

python3 - <<'PY'
import json
import sys

compact = json.load(open("compact.json", encoding="utf-8"))
print(f"Compact command: {compact.get('local_command')}")
if compact.get("local_command") != "compact":
    print("FAIL: manual compaction was not observed.", file=sys.stderr)
    sys.exit(1)
print("PASS: manual compaction ran.")
PY

debug_file=$(ls -t "$HOME"/.claude/debug/*.txt | head -1)
printf 'Claude Code debug log: %s\n' "$debug_file"
if ! rg -Fq 'session.compact bridge probe' "$debug_file"
then
  printf 'FAIL: bridge probe did not log; inspect hook registration in the debug log.\n' >&2
  exit 1
fi
printf 'PASS: bridge probe logged its route results.\n'
