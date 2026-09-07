#!/usr/bin/env bash
# Drop all permissions matching PATTERN from ~/.claude/settings.json.
# Useful for simulating permission drift in smoke tests.
# Usage: scripts/drop-perm.sh PATTERN
#
# Example: scripts/drop-perm.sh "Bash.git"

set -euo pipefail

pattern="${1:?Usage: drop-perm.sh PATTERN}"
settings="$HOME/.claude/settings.json"

if test ! -f "$settings"
then echo "settings.json not found: $settings" >&2
     exit 1
fi

before=$(jq '.permissions.allow | length' "$settings")
jq --arg pat "$pattern" \
    '.permissions.allow |= map(select(test($pat) | not))' \
    "$settings" > "$settings.tmp"
mv "$settings.tmp" "$settings"
after=$(jq '.permissions.allow | length' "$settings")
echo "Dropped $(( before - after )) permission(s) matching: $pattern"
