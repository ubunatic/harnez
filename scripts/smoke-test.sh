#!/usr/bin/env bash
# Smoke-test the apply/diff/clean workflow.
# Run from the project root: scripts/smoke-test.sh

set -euo pipefail

bin="./claudeconfig"

pass() { echo "  PASS: $*"; }
fail() { echo "  FAIL: $*" >&2; exit 1; }

echo "=== build ==="
make build

echo ""
echo "=== apply ==="
"$bin" apply

echo ""
echo "=== apply again: must be idempotent ==="
out=$("$bin" apply 2>&1)
printf '%s\n' "$out"
if printf '%s\n' "$out" | grep -q "No changes\."
then
    pass "idempotent"
else
    fail "second apply made unexpected changes"
fi

echo ""
echo "=== simulate drift: drop git permission ==="
scripts/drop-perm.sh "Bash.git"

echo ""
echo "=== apply after drift: must restore ==="
out=$("$bin" apply 2>&1)
printf '%s\n' "$out"
if printf '%s\n' "$out" | grep -q "permissions.allow"
then
    pass "drift repaired"
else
    fail "drift not repaired"
fi

echo ""
echo "=== apply after repair: must be idempotent ==="
out=$("$bin" apply 2>&1)
printf '%s\n' "$out"
if printf '%s\n' "$out" | grep -q "No changes\."
then
    pass "idempotent after repair"
else
    fail "apply after repair made unexpected changes"
fi

echo ""
echo "All smoke tests passed."
