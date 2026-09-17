#!/usr/bin/env bash
# scripts/canary-visual-doc/run.sh - Execute visual cheatsheet subagent canary
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
SCRIPT_DIR="$REPO_ROOT/scripts/canary-visual-doc"
VISION_DIR="$REPO_ROOT/scratch/vision"

printf '==> Building visual doc canary binary...\n'
go build -o "$SCRIPT_DIR/canary-visual-doc" "$SCRIPT_DIR/main.go"

IMG="${1:-$VISION_DIR/Bash_2col.png}"
TASK="${2:-$SCRIPT_DIR/fixtures/bash-deploy-check.task.md}"
OUT="${3:-deploy-check.sh}"
BASELINE="${4:-$REPO_ROOT/docs/lang/Bash.md}"

if ! test -f "$IMG"
then
	printf 'Cheatsheet image %s not found. Generating visual assets first...\n' "$IMG"
	python3 "$REPO_ROOT/scripts/canary-doc-vision/render.py" \
		"$REPO_ROOT/docs/lang/Bash.md" \
		--outdir "$VISION_DIR" \
		--layouts 2col \
		--formats png
fi

printf '==> Executing Canary...\n'
"$SCRIPT_DIR/canary-visual-doc" \
	-baseline "$BASELINE" \
	-image "$IMG" \
	-task "$TASK" \
	-out "$OUT" \
	"${@:5}"
