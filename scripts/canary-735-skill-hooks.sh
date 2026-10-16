#!/usr/bin/env bash
set -euo pipefail

# Canary for issue 735 M3: which Claude Code hook payloads carry skill calls?
# Runs headless Claude (haiku) in a scratch dir with hooks passed by
# --settings only; ~/.claude is not changed. Each hook appends its stdin
# (one JSON object per line) to $dir/events.jsonl.
#
#   scripts/canary-735-skill-hooks.sh [dir]

dir="${1:-$(mktemp -d /tmp/canary-735.XXXXXX)}"
mkdir -p "$dir/.claude/skills/zz735"
cat > "$dir/.claude/skills/zz735/SKILL.md" <<'MD'
---
name: zz735
description: Canary skill for issue 735. Use only when asked to run zz735.
---
Reply with exactly ZZ735_OK and nothing else.
MD
log="$dir/events.jsonl"
hook="cat >> $log; echo >> $log"
cat > "$dir/settings.json" <<JSON
{"hooks": {
  "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "$hook"}]}],
  "PreToolUse": [{"matcher": "Skill", "hooks": [{"type": "command", "command": "$hook"}]}],
  "PostToolUse": [{"matcher": "Skill", "hooks": [{"type": "command", "command": "$hook"}]}],
  "PostToolUseFailure": [{"matcher": "Skill", "hooks": [{"type": "command", "command": "$hook"}]}]
}}
JSON
run() {
    echo "== $1"
    (cd "$dir" && claude -p --model haiku --settings "$dir/settings.json" \
        --allowedTools Skill <<<"$1" 2>&1 | head -c 300) || true
    echo
}
run "/zz735 some args"
run "Use the Skill tool to run the skill zz735."
run "Use the Skill tool with skill name zz735-missing exactly once, then stop."
run "/clear"
run "/model"
run "/compact"
run "/tmp/a.md what is this path?"
echo "events: $log"
