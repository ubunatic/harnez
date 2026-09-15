#!/usr/bin/env bash
set -euo pipefail

# canary-lite-doc/run.sh — behavioral canary for a lite doc variant.
#
# Spawns a genuinely isolated `claude -p` session (a scratch directory
# outside any harnez-managed project, so no local AGENTS.md/CLAUDE.md is
# auto-injected) whose ONLY context is the given lite doc, hands it a coding
# task exercising that doc's rules, then runs `harnez lint --check` on the
# real output file as the mechanical judge — no custom LLM-response scoring
# needed. See issue 362.
#
# Usage: run.sh <lite-doc-path> <task-file> <output-filename>
# Example:
#   run.sh docs/lang/Bash.lite.md scripts/canary-lite-doc/fixtures/bash-deploy-check.task.md deploy-check.sh

doc="${1:?Usage: run.sh <lite-doc-path> <task-file> <output-filename>}"
task="${2:?Usage: run.sh <lite-doc-path> <task-file> <output-filename>}"
out_name="${3:?Usage: run.sh <lite-doc-path> <task-file> <output-filename>}"

doc_abs="$(cd "$(dirname "$doc")" && pwd)/$(basename "$doc")"
task_abs="$(cd "$(dirname "$task")" && pwd)/$(basename "$task")"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if test ! -f "$doc_abs"
then printf 'ERROR: lite doc not found: %s\n' "$doc_abs" >&2
     exit 1
fi
if test ! -f "$task_abs"
then printf 'ERROR: task file not found: %s\n' "$task_abs" >&2
     exit 1
fi

work=$(mktemp -d /tmp/canary-lite-doc.XXXXXX)
trap 'rm -rf "$work"' EXIT

cp "$doc_abs" "$work/STYLE.md"
task_text=$(cat "$task_abs")

prompt="You are in an empty directory with one file, STYLE.md — that is your
only style guide, and the only context you have. Read STYLE.md, then
complete this task:

${task_text}

Write the file to disk, then reply with only the file path and no other text."

printf 'Running isolated agent in %s ...\n' "$work"
out_path=$(cd "$work" && claude -p --permission-mode bypassPermissions "$prompt" | tail -1)

if test ! -f "$out_path"
then printf 'FAIL: agent did not produce a file at reported path %s\n' "$out_path" >&2
     exit 1
fi

printf '\n=== generated file: %s ===\n' "$out_path"
cat "$out_path"

printf '\n=== harnez lint --check ===\n'
if "$repo_root/harnez" lint --check "$out_path"
then printf 'PASS: %s (doc=%s, task=%s)\n' "$out_name" "$doc" "$task"
else printf 'FAIL: lint findings for %s (doc=%s, task=%s)\n' "$out_name" "$doc" "$task" >&2
     exit 1
fi
