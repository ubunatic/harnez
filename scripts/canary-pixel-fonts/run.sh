#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Uwe Jugel
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCRATCH_DIR="$PROJECT_ROOT/scratch/pixel-fonts"

mkdir -p "$SCRATCH_DIR"

echo "=== 1. Running Python Pixel Font Canary Probe ==="
python3 "$SCRIPT_DIR/probe.py"

echo "=== 2. Running Go Pixel Font Benchmark ==="
go run "$SCRIPT_DIR/main.go" --benchmark

echo "=== 3. Rendering Sample 256x256, 384x384, and 512x512 Test Cards ==="
go run "$SCRIPT_DIR/main.go" --font 5x8 --scale 1 --width 256 --height 256 --out "$SCRATCH_DIR/go_5x8_256.png"
go run "$SCRIPT_DIR/main.go" --font 3x5 --scale 1 --width 256 --height 256 --out "$SCRATCH_DIR/go_3x5_256.png"
go run "$SCRIPT_DIR/main.go" --font 5x8 --scale 2 --width 512 --height 512 --out "$SCRATCH_DIR/go_5x8_s2_512.png"

echo "=== Canary Complete. Artifacts in $SCRATCH_DIR ==="
ls -lh "$SCRATCH_DIR"
