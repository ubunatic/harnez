#!/usr/bin/env bash
# Runs the Pi background-task extension tests (test/pi), which import the TypeScript sources directly.
# Node 22.18 or newer is required: it runs TypeScript through built-in type stripping. A missing or older
# node fails this script; the tests are never skipped for it.
# Run from the project root: scripts/test-pi.sh

set -euo pipefail

if ! command -v node >/dev/null 2>&1
then echo "test-pi: node is required (22.18 or newer) to run test/pi" >&2
     exit 1
fi
if ! node -e 'const [major, minor] = process.versions.node.split(".").map(Number); process.exit(major > 22 || (major === 22 && minor >= 18) ? 0 : 1)'
then echo "test-pi: node $(node --version) is too old; 22.18 or newer is required" >&2
     exit 1
fi

node --import ./test/pi/register.mjs --test test/pi/background-tasks.test.ts test/pi/background-claim.test.ts
