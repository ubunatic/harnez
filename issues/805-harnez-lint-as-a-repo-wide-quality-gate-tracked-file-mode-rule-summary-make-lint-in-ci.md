# 805 — harnez lint as a repo-wide quality gate: tracked-file mode, rule summary, make lint in CI

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Tooling
**Related**: 276, 057

---

## 1. Problem & Motivation
`harnez lint` requires explicit file args, so it cannot act as a repo-wide gate; `make lint` only checks docs/commands registration. Running it over all tracked go/sh/md/Makefile files reports 40 findings: bash dangling-then/strict-mode in scripts and canaries, and marker-integrity false positives where issues/studies *mention* `harnez:begin` markers in prose or inline code.

## 2. Technical Specification / Findings
- No args → lint `git ls-files` (go, sh, md, Makefile/mk); directory args → tracked files below the dir.
- Summary by rule (count per rule ID) in text output.
- `--check` exits non-zero only for findings at or above the check level; without `--check` exit 0.
- Marker rule: ignore markers inside inline code spans and fenced blocks (prose mentions are not markers).

## 3. Implementation & Verification Plan
- Unit tests for file discovery, summary and exit codes.
- Fix the bash findings in scripts/canaries; justify any remaining.
- `make lint` = gofmt check + go vet + `harnez lint --check` + scripts/lint.sh; CI already runs `make lint`.
