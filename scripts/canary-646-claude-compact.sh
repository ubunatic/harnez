#!/usr/bin/env bash
set -euo pipefail

# Manual canary for issue 646: does Claude Code's session.compact function hook
# fire on an interactive /compact and replace the conversation?
#
#   scripts/canary-646-claude-compact.sh         start Claude in a scratch dir
#   scripts/canary-646-claude-compact.sh check   show the result of the last run
#
# In the session: chat briefly, run /compact, then ask "what is the marker?".
# Pass: Claude answers HARNEZ_646_REPLACEMENT_7f3a91c2; check finds it in the session file.
# Your ~/.claude settings are not changed; the flag and plugin apply to this run only.

root="$(cd "$(dirname "$0")/.." && pwd)"
plugin="$root/canary/646-session-compact"
state="${XDG_CACHE_HOME:-$HOME/.cache}/harnez/canary-646-last-dir"
marker="HARNEZ_646_REPLACEMENT_7f3a91c2"

check() {
    if ! test -f "$state"
    then echo "No run recorded; start one first." >&2
         exit 1
    fi
    dir="$(cat "$state")"
    sessions="$HOME/.claude/projects/$(echo "$dir" | tr '/.' '--')"
    echo "Run dir:  $dir"
    echo "Sessions: $sessions"
    if ! test -d "$sessions"
    then echo "FAIL: no session saved for this run"
         exit 1
    fi
    if grep -q compact_boundary "$sessions"/*.jsonl
    then echo "  compaction ran"
    else echo "FAIL: no compaction recorded (did you run /compact?)"
         exit 1
    fi
    if grep -q "$marker" "$sessions"/*.jsonl
    then echo "PASS: hook replaced the conversation with $marker"
    else echo "FAIL: marker missing, Claude used its built-in summary"
    fi
}

if test "${1:-}" = "check"
then check
     exit 0
fi

if ! test -f "$plugin/hooks/probe.ts"
then echo "Canary plugin missing: $plugin" >&2
     exit 1
fi

dir="$(mktemp -d -t harnez-canary-646-XXXX)"
mkdir -p "$(dirname "$state")"
echo "$dir" > "$state"
echo "Scratch dir: $dir"
echo "Steps: say hello, run /compact, ask 'what is the marker?', exit, then run: $0 check"
cd "$dir"
CLAUDE_CODE_ENABLE_FUNCTION_HOOKS=1 claude --plugin-dir "$plugin"
