# 520 — TestResolveExecTimeout fails when HTO is set in the environment

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Testing
**Related**: [[506-hto-harnez-timeout-intent-prefix]], [[509-session-tip-leaks-into-tests]]

## Problem

During the 519 M1 run (2026-09-24), `make test-q1` failed
`TestResolveExecTimeout_DefaultFlagAndExplicitPrefix` in `cmd/harnez` because the
developer's environment had `HTO=0`. The test reads the ambient environment.

## /goal

The test passes regardless of `HTO` / `HARNEZ_TIMEOUT` in the caller's environment
(clear them with `t.Setenv`), and an explicit case covers an ambient value.
