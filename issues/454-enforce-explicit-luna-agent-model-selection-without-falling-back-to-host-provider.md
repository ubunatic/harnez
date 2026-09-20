# 454 — Enforce explicit luna agent model selection without falling back to host provider

**Status**: Open

---

Reserved placeholder ticket.
# 454 — Enforce explicit luna agent model selection without falling back to host provider

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Agentic Ergonomics
**Related**: `AGENTS.md`, `@docs/AgenticLoop.md`

## Problem

When the user or orchestration loop specifies `luna` (for example,
`codex:luna:low` or another low-cost worker tier), agents can substitute their
host model or a native subagent instead of dispatching the requested external
model through Harnez. This violates explicit cost and tier boundaries.

## Goal

Guarantee that an explicit model/provider specification is dispatched exactly as
requested via `harnez agent start <provider:model:tier>`. If that provider or
model is unavailable or unconfigured, fail clearly and ask for guidance rather
than silently substituting a host model.

The implementation must preserve strict mapping for specifications such as
`codex:luna:low`, and must include coverage for the unavailable-provider and
no-silent-fallback cases.
