# 493 — Add mixed subagent dispatch mode and make sprint skills dispatch-mode aware

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 490, 491, 435, `docs/HarnezComponents.md` §8.9, `docs/HarnezAgentArchitecture.md` §5

## /goal

Extend `subagent_mode` (`internal/agentpolicy`, today `harnez|native`) with `mixed`, and
make sprint skills follow the effective mode instead of hard-coding `harnez agent start`.

## 1. Problem & Motivation

Sprint skills (`sprint`, `lean-sprint`, `reverse-sprint`) name `harnez agent start`
directly. A `docs-only` user, or a repo with `subagent_mode: native`, gets instructions for
a dispatcher it does not use. Gating the skills with `requires: [agents]` would remove them
for native users who can run them fine. Decision (§8.9): keep the skills, make dispatch an
opt-in mode.

## 2. Technical Specification

| Mode | Dispatch |
|---|---|
| `native` | harness-native subagents only |
| `harnez` | every subagent through `harnez agent` |
| `mixed` | native for the host's own vendor (Claude → Claude), `harnez agent` for other vendors (Claude → Codex) |

- `agentpolicy.Configure`/`Resolve` accept `mixed`; `harnez agent` interception honours it
  (same-vendor requests are passed back to native dispatch).
- Effective mode = policy mode bounded by the component selection: `agents` disabled ⇒
  `native` (491).
- Sprint skill text: one dispatch paragraph per mode, or a mode-neutral instruction that
  points at the effective policy block; no unconditional `harnez agent start`.

## 3. Implementation & Verification Plan

- [ ] `mixed` in agentpolicy, with tests for Configure/Resolve/conflict reporting
- [ ] Interception behaviour for `mixed` (vendor of host vs. requested model)
- [ ] Sprint skills rewritten mode-aware; docs in HarnezAgentArchitecture.md §5
- [ ] Effective-mode clamp to `native` when `agents` is disabled (after 491)
