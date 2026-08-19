#!/usr/bin/env bash
# Smoke-test the apply/diff/clean workflow.
# Run from the project root: scripts/smoke-test.sh

set -euo pipefail

bin="./harnez"

pass() {
	echo "  PASS: $*"
}
fail() {
	echo "  FAIL: $*" >&2
	exit 1
}

echo "=== test ==="
make test

echo ""
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
echo "=== diff under drift: assert exit code 1 with --exit-code ==="
if "$bin" diff --exit-code >/dev/null 2>&1
then
    fail "diff --exit-code exited 0 despite drift"
else
    pass "diff --exit-code returned non-zero on drift"
fi

echo ""
echo "=== diff under drift: assert exit code 0 without flag ==="
if "$bin" diff >/dev/null 2>&1
then
    pass "diff without flag exited 0 under drift"
else
    fail "diff without flag returned non-zero under drift"
fi

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
echo "=== diff after repair: assert exit code 0 with --exit-code ==="
if "$bin" diff --exit-code >/dev/null 2>&1
then
    pass "diff --exit-code exited 0 after repair"
else
    fail "diff --exit-code exited non-zero after repair"
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
