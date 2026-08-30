#!/usr/bin/env bash
set -euo pipefail

# Canary for driving `harnez usage --watch` under a real pseudo-terminal from
# a non-interactive shell. --watch reads /dev/tty directly for keypresses and
# runs `stty` against it, so a plain pipe/redirect leaves it unable to detect
# a terminal at all — this uses util-linux's `script` to allocate a real pty,
# the same mechanism an interactive terminal emulator provides, letting a
# script or CI capture --watch's live redraw output for inspection. See
# docs/Canary.md for the canary-first convention this follows.
#
# Usage: scripts/canary-watch-pty.sh SECONDS [EXTRA_HARNEZ_USAGE_ARGS...]
# Example: scripts/canary-watch-pty.sh 30 --proc

seconds="${1:?Usage: scripts/canary-watch-pty.sh SECONDS [EXTRA_HARNEZ_USAGE_ARGS...]}"
shift
log=$(mktemp /tmp/harnez-watch-pty-XXXXXX.log)

echo "==> Recording ${seconds}s of 'harnez usage --watch $*' under a pty"
timeout "${seconds}" script -qec "COLUMNS=200 LINES=60 harnez usage --watch $*" "${log}" >/dev/null 2>&1 || true

echo "==> Panel titles seen (deduplicated, in first-seen order):"
grep -ao '\[[A-Za-z]\][^[:cntrl:]]*' "${log}" |
    sed -E 's/\x1b\[[0-9;]*m//g' |
    sort -u

echo "==> Remote Load box label occurrences (streaming vs batch, for flap detection):"
grep -aoE '\(@[^ )]+ · (streaming|batch)\)' "${log}" |
    sed -E 's/\x1b\[[0-9;]*m//g' |
    sort |
    uniq -c

echo ""
echo "==> Canary completed. Full capture kept at: ${log}"
