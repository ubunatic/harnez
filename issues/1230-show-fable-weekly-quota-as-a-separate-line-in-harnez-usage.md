# 1230 — Show Fable weekly quota as a separate line in harnez usage

**Status**: Closed — f8ff825e + tests: Fable weekly line in table, compact, detailed and JSON (extra_windows.fable); live-verified 2026-11-13
**Closed-By**: session 77baa296-b04d-4941-bc73-3791a6b9b1e3
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 648, 485

---

## 1. Problem & Motivation
`claude -p /usage` now reports a separate per-model weekly limit, e.g. `Current week (Fable): 96% used · resets Nov 14, 5:59pm`, besides `Current week (all models)`. `harnez usage` and `harnez usage --minimal -w` show only the all-models weekly quota, so a nearly exhausted Fable quota (96% while another model has headroom) stays invisible. This matters for picking agent models such as `claude:fable:*` (`harnez agent models`).

## 2. Technical Specification / Findings
- Source: the `Current week (Fable)` line of `claude -p /usage`; it has its own reset time.
- Show it as its own line in the default usage view and in `harnez usage --minimal -w`, labelled Fable, next to the weekly line.
- Omit the line when Claude reports no Fable limit. No fake zero.
- Also include it in `--json` as a separate field (check the schema in `spec/`).
- Re-verify the current usage collector and parser against live code first.

## 3. Implementation & Verification Plan
- Extend the Claude usage parser to capture the Fable weekly window, with a fixture test from the sample line above.
- Render the line in both views; add render tests (present and absent).
- Live check: compare `harnez usage` and `harnez usage --minimal -w` with `claude -p /usage`.

**/goal** Show the Fable weekly quota as a separate line in `harnez usage` and `harnez usage --minimal -w`, verified by tests and a live check, or stop and report when blocked on a user decision or denied permission.
