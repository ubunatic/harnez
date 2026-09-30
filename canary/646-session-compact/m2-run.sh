#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "$0")" && pwd)
repo_dir=$(cd "$script_dir/../.." && pwd)
run_dir=$(mktemp -d "${TMPDIR:-/tmp}/harnez-646-m2-XXXXXX")
cleanup() {
  status=$?
  if test "$status" -eq 0
  then rm -rf "$run_dir"
  else printf 'M2 canary artifacts retained: %s\n' "$run_dir" >&2
  fi
  return "$status"
}
trap cleanup EXIT

claude_dir="$run_dir/claude"
config_file="$run_dir/config.yaml"
debug_file="$run_dir/claude-debug.log"
first_json="$run_dir/first.json"
compact_json="$run_dir/compact.json"
harnez_version=$(harnez --version | awk '{print $3}')
plugin_dir="$claude_dir/plugins/cache/harnez-local/jev-compaction/$harnez_version"

python3 - "$repo_dir/config.yaml" "$config_file" "$run_dir" <<'PY'
from pathlib import Path
import sys

source, destination, root = map(Path, sys.argv[1:])
config = source.read_text()
config = config.replace('jev_compaction_enabled: false', 'jev_compaction_enabled: true', 1)
config = config.replace('prime_agent_target: ~/.prime/agent', f'prime_agent_target: {root}/prime', 1)
destination.write_text(config)
PY

harnez apply -c "$config_file" -t "$claude_dir" --components usage --no-docs \
  --agy-target "$run_dir/agy" --codex-target "$run_dir/codex" > "$run_dir/apply.log"

cd "$run_dir"
CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1 claude -p \
  --model haiku --dangerously-skip-permissions --allowedTools Bash --plugin-dir "$plugin_dir" \
  --output-format json --debug-file "$debug_file" \
  "Run these 10 Bash commands one at a time. Each command must print 1200 lines in the format 'HARNEZ646 <command-number> <line-number> ' followed by 40 repeated x characters. Do not summarize the outputs until all ten commands have run. Then reply READY." > "$first_json"

session_id=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["session_id"])' "$first_json")
CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1 claude -p \
  --model haiku --dangerously-skip-permissions --allowedTools Bash --plugin-dir "$plugin_dir" \
  --output-format json --debug-file "$debug_file" --resume "$session_id" \
  '/compact' > "$compact_json"

if ! rg -Fq 'session.compact settled' "$debug_file"
then printf 'FAIL: session.compact hook did not settle; see %s\n' "$debug_file" >&2
     exit 1
fi
if ! rg -Fq 'harnez compaction applied:' "$debug_file"
then
  fallback=$(rg -F 'harnez compaction fallback:' "$debug_file" | tail -1 || true)
  printf 'FAIL: no Harnez replacement was applied. Fallback: %s\n' "${fallback:-none reported}" >&2
  exit 1
fi

python3 - "$session_id" <<'PY'
import json
from pathlib import Path
import sys

session_id = sys.argv[1]
transcripts = list((Path.home() / '.claude/projects').rglob(f'{session_id}.jsonl'))
if not transcripts:
    raise SystemExit('FAIL: Claude session transcript not found under ~/.claude/projects')
boundaries = []
for transcript in transcripts:
    for line in transcript.read_text(errors='replace').splitlines():
        try:
            item = json.loads(line)
        except json.JSONDecodeError:
            continue
        if item.get('subtype') == 'compact_boundary':
            boundaries.append(item)
if not boundaries:
    raise SystemExit('FAIL: compact_boundary record missing from session transcript')
boundary = boundaries[-1]
metadata = boundary.get('compactMetadata', {})
pre, post = metadata.get('preTokens'), metadata.get('postTokens')
if not isinstance(pre, int) or not isinstance(post, int):
    raise SystemExit(f'FAIL: boundary token counts unavailable: {boundary}')
print(f'M2 real harnez compact replacement: before={pre} tokens, after={post} tokens')
PY

rg -F 'harnez compaction applied:' "$debug_file" | tail -1
printf 'Session id: %s\n' "$session_id"
