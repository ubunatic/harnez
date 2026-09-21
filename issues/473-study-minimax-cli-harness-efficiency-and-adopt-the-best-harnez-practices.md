# 473 — Study MiniMax CLI harness efficiency and adopt the best Harnez practices

**Status**: Open
**Priority**: P2
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: MiniMax-AI/minimax-code; `docs/AgenticLoop.md`; `cmd/harnez/agent`

---

## Problem & Motivation

MiniMax Code presents a notably efficient terminal harness with distinct
interactive and headless execution, resumable sessions, workspace guidance,
provider/model capability checks, and a compact reproducible task loop. Determine
which of these practices improve Harnez rather than copying product-specific
behavior or relying on marketing claims.

## Goal

Produce an evidence-based comparison and adopt the highest-value compatible
practices in Harnez through focused commands, defaults, or documentation. The
work is done when each proposed practice has a fit decision, an implementation
or explicit rejection rationale, and verification criteria; any accepted code
changes include focused tests and updated workflow guidance.

## Findings to Validate

- Separate interactive, headless, and editor-facing execution contracts.
- Make session continuation and workspace-scoped context first-class.
- Validate provider/model capabilities before starting a task when possible.
- Keep a small, deterministic build-test-fix example for harness regression.
- Measure efficiency as correctness, latency, tool calls, and token/context cost,
  not only benchmark scores.

## Implementation & Verification Plan

Inspect the current Harnez agent/session/exec surfaces and MiniMax Code source and
documentation; compare behavior with a small canary task; then file or implement
the smallest worthwhile improvements. Verify with focused CLI tests, the existing
agent canaries, and documented before/after measurements where available.

**Status**: Draft

---

Reserved placeholder ticket.
