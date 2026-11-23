# 1269 — TestHelpGolden fails on main: help.golden misses pr wait --fixup text from b18d7805

**Status**: Closed — help.golden regenerated in f28f0f4d (pr wait --fixup text); TestHelpGolden green in 1045 M2-M4 Quota-1 runs
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1261, 1045

---

## 1. Problem & Motivation
`make test-q1` fails on `TestHelpGolden` independently of the change under test, so every Quota-1 run reports a red `cmd/harnez` and hides real failures. Observed in the 1045 M1 run (2026-11-22, log `test-20261122T213550…`):

```
help_golden_test.go:70: help output differs from testdata/help.golden at line 2591:
     got: "--fixup handles the common conflict itself: one only in the generated"
    want: "--contains asks the forge's compare API first and falls back to git fetch"
```

## 2. Technical Specification / Findings
- `cmd/harnez/pr.go` on main has the `--fixup` help text (b18d7805, issue 1261). `cmd/harnez/testdata/help.golden` lacks it.
- Both files are identical on main and on `sprint/1045`, so the failure predates 1045 (static check; not yet run on a clean main checkout).

## 3. Implementation & Verification Plan
- Regenerate the golden with the test's update mechanism, review that the diff is only the `pr wait --fixup` help text, and commit.
- Verify: `TestHelpGolden` passes in the next Quota-1 run.
