#!/usr/bin/env bash
# run.sh - Run markdown screenshot rendering and vision token benchmarks
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
SCRIPT_DIR="$REPO_ROOT/scripts/canary-doc-vision"
OUT_DIR="$REPO_ROOT/scratch/vision"

mkdir -p "$OUT_DIR"

printf '==> Running Doc Vision Rendering and Benchmark Harness\n'

python3 "$SCRIPT_DIR/render.py" \
  "$REPO_ROOT/docs/practices/AgenticLoop.md" \
  "$REPO_ROOT/docs/lang/Bash.md" \
  --outdir "$OUT_DIR" \
  --layouts 1col 2col 3col \
  --formats png jpg

printf '\n==> Vision Benchmark Complete. Artifacts saved in %s\n' "$OUT_DIR"
