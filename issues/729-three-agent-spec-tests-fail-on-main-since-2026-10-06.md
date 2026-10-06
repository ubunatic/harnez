# 729 — Three agent-spec tests fail on main since 2026-10-06

**Status**: Closed — stale tests now read the default model from spec/agent.yaml (6e976d52); make test-q1 green
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Bug
**Related**: `spec/agent.yaml`, issues/715 (terra removal, `4f98e462`), issues/711 (cost refresh)

---

## 1. Problem & Motivation

`make test-q1` fails on main. Confirmed on `2f4f946e`, before issue 726 started, so 726 did not
cause them:

- `TestAgentStartDefaultModelLine` (cmd/harnez)
- `TestFlash38EscalationGuidanceAndLeanSprintDeveloperPreference`
- `TestEmbeddedAgentSpecLoadsAndResolves`

A red suite hides new regressions; every sprint now has to carry a known-failures list.

## 2. Technical Specification / Findings

All three concern the subagent model spec. Recent `spec/agent.yaml` changes: `4f98e462` (remove
terra, issue 715), `fe71de23` and `6cbf3e4e` (cost refresh, issue 711). Unverified which one broke
them; bisect first.

## 3. Implementation & Verification Plan

/goal `make test-q1` is green on main, with each test fixed to match the intended spec (not
loosened). Stop and report when the intended spec value is unclear and needs a user decision.

## 4. Results

Cause: `6310ccae` (model cost research, 2026-10-05) deliberately changed
`default_model` to `codex:luna:high` and rewrote the `agy:flash38` guidance;
the tests still asserted the old values. Fixed in `6e976d52`: the default-model
tests read `spec/agent.yaml` instead of a literal, and the agy test keeps its
role checks but drops the flash38 wording check. `make test-q1` is green.
