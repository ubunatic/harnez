#!/usr/bin/env bash
set -euo pipefail

plugin_dir=$(cd "$(dirname "$0")" && pwd)
run_dir=$(mktemp -d "${TMPDIR:-/tmp}/harnez-646-XXXXXX")
trap 'rm -rf "$run_dir"' EXIT

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

CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1 claude -p --model haiku \
  --plugin-dir "$plugin_dir" --output-format json --resume "$session_id" \
  'What exact replacement marker is present in the compacted context? Reply with the marker only.' > recall.json

python3 - <<'PY'
import json
import sys

marker = "HARNEZ_646_REPLACEMENT_7f3a91c2"
compact = json.load(open("compact.json", encoding="utf-8"))
recall = json.load(open("recall.json", encoding="utf-8"))

print(f"Compact command: {compact.get('local_command')}")
print(f"Replacement marker response: {recall.get('result')}")
if compact.get("local_command") != "compact" or recall.get("result", "").strip() != marker:
    print("FAIL: manual compaction or replacement marker was not observed.", file=sys.stderr)
    sys.exit(1)
print("PASS: manual compaction returned the hook-only marker to the resumed model.")
PY

printf '\nRun artifacts were removed with the scratch directory.\n'
