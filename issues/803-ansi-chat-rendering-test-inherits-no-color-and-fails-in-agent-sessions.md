# 803 — ANSI chat rendering test inherits NO_COLOR and fails in agent sessions

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: Discovered while verifying 802.

---

## 1. Problem & Motivation
Goal: make ANSI color assertions deterministic in agent sessions and CI.
`make test-q1` fails in `TestChatRendersANSIToolOutput` when the caller exports
`NO_COLOR=1`: both the first wrapped cell and the reset cell render without color.

## 2. Technical Specification / Findings
Loom v0.3.10 honors `NO_COLOR` when rendering a canvas. The test asserts green
foreground cells but does not select a color profile; the session environment
therefore changes its outcome. The finder regressions and all other packages passed.

## 3. Implementation & Verification Plan
Select `LOOMCOLOR=256` inside this color-specific test with `t.Setenv`, retaining
all existing assertions. Run the full `make test-q1` suite under the original
`NO_COLOR=1` environment, then the requested container check.
