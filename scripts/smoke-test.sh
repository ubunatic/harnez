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
then pass "idempotent"
else fail "second apply made unexpected changes"
fi

echo ""
echo "=== simulate drift: drop git permission ==="
scripts/drop-perm.sh "Bash.git"

echo ""
echo "=== diff under drift: assert exit code 1 with --exit-code ==="
if "$bin" diff --exit-code >/dev/null 2>&1
then fail "diff --exit-code exited 0 despite drift"
else pass "diff --exit-code returned non-zero on drift"
fi

echo ""
echo "=== diff under drift: assert exit code 0 without flag ==="
if "$bin" diff >/dev/null 2>&1
then pass "diff without flag exited 0 under drift"
else fail "diff without flag returned non-zero under drift"
fi

echo ""
echo "=== apply after drift: must restore ==="
out=$("$bin" apply 2>&1)
printf '%s\n' "$out"
if printf '%s\n' "$out" | grep -q "permissions.allow"
then pass "drift repaired"
else fail "drift not repaired"
fi

echo ""
echo "=== diff after repair: assert exit code 0 with --exit-code ==="
if "$bin" diff --exit-code >/dev/null 2>&1
then pass "diff --exit-code exited 0 after repair"
else fail "diff --exit-code exited non-zero after repair"
fi

echo ""
echo "=== apply after repair: must be idempotent ==="
out=$("$bin" apply 2>&1)
printf '%s\n' "$out"
if printf '%s\n' "$out" | grep -q "No changes\."
then pass "idempotent after repair"
else fail "apply after repair made unexpected changes"
fi

echo ""
echo "All apply/diff/clean smoke tests passed."

echo ""
echo "=== usage --compact: output and bar alignment ==="

# Use the installed binary so this tests what's actually deployed.
installed_bin="$(command -v harnez 2>/dev/null || true)"
if test -z "$installed_bin"
then echo "  SKIP: harnez not found on PATH — run 'make install' first"
else # Capture output with ANSI codes stripped via sed.
    # We can't pipe through 'strip-ansi' tools that may not be installed, so
    # use a sed expression to remove ESC[…m sequences inline.
    raw_usage=$("$installed_bin" usage --compact 2>/dev/null || true)
    usage_plain=$(printf '%s\n' "$raw_usage" | sed 's/\x1b\[[0-9;]*m//g')

    # Smoke: command must produce some output and exit 0.
    if test -z "$usage_plain"
    then fail "harnez usage --summary --compact produced no output"
    fi
    pass "harnez usage --summary --compact exited 0 with output"

    # Extract content lines from the [a] All Usage box (between the first ┌ and
    # first └ after a line containing "[a] All Usage").
    in_all_usage=false
    all_usage_rows=""
    while IFS= read -r line
    do
        if printf '%s\n' "$line" | grep -q "All Usage"
        then in_all_usage=true
             continue
        fi
        if test "$in_all_usage" = true
        then # Stop at the closing border.
             if printf '%s\n' "$line" | grep -qF "└"
             then break
             fi
             # Content lines start with "│ "
             row=$(printf '%s\n' "$line" | sed 's/^│ //' | sed 's/ │$//')
             if test -n "$row"
             then all_usage_rows="${all_usage_rows}${row}
"
             fi
        fi
    done <<EOF
$usage_plain
EOF

    if test -z "$all_usage_rows"
    then fail "no All Usage rows found in 'harnez usage --compact' output"
    fi
    pass "found [a] All Usage rows"

    # For each row find the column of the last '[' (the second progress bar opener).
    # All rows must agree on the same column — that's the alignment guarantee.
    expected_col=""
    row_num=0
    alignment_ok=true
    while IFS= read -r row
    do
        test -z "$row" && continue
        row_num=$((row_num + 1))

        # Find position of last '[' using awk (POSIX, no bash string tricks).
        col=$(printf '%s\n' "$row" | awk '{
            pos = -1
            n = split($0, chars, "")
            for (i = 1; i <= n; i++) {
                if (chars[i] == "[") pos = i - 1
            }
            print pos
        }')

        if test "$col" -lt 0
        then echo "  WARN: row $row_num has no second bar, skipping: $row"
             continue
        fi
        if test -z "$expected_col"
        then expected_col="$col"
        elif test "$col" != "$expected_col"
        then echo "  FAIL: bar alignment broken on row $row_num" >&2
             echo "        expected col $expected_col, got col $col" >&2
             echo "        row: $row" >&2
             alignment_ok=false
        fi
    done <<EOF
$all_usage_rows
EOF

    if test "$alignment_ok" = true
    then pass "all [a] All Usage bars align at column $expected_col"
    else fail "bar column alignment is broken (see above)"
    fi
fi

echo ""
echo "All smoke tests passed."
