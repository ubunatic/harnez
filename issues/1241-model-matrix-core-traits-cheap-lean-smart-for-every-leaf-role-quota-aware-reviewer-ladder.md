# 1241 — Model matrix: core traits cheap/lean/smart for every leaf role, quota-aware reviewer ladder

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 983 (role-scoped picks), 1059 (slash picks in /lean-sprint)

---

## 1. Problem & Motivation

- Traits are defined per role: developer has eight, advisor `cheap`/`smart`, reviewer only `smart`.
  `--role reviewer --model lean` is rejected, and `/lean-sprint` documented exactly that call as the
  reviewer default. This has hit us more than once.
- The reviewer `smart` chain was `[sol:low]`. Fallback skips exhausted or unsupported models
  (`ResolveModelWithOptions`, `internal/subagent/driver.go`), but a one-entry chain has nothing to
  fall back to, so a reviewer start fails whenever Sol is unavailable.

## 2. Technical Specification / Findings

Owner tiers (2026-11-17, "roughly, I change my mind now and then"; user `models.yaml` overrides
them per machine):

| Tier | Trait | Models |
|---|---|---|
| smart but cheap | cheap | haiku:high, luna:high |
| smart and inexpensive | lean | terra:med, hydrogen:low |
| smart | smart | sol:med, hydrogen:med, opus:med |
| smartest | best | sol:high, hydrogen:high, argon:high (own quota), opus:high, fable:high (own quota); astra:high avoided |

- Reviewer: all four tiers, default `smart`.
- Advisor: gains `lean` from the same row; `cheap` and `smart` unchanged.
- Developer: unchanged except the empty `ai` trait, which is dropped.
- `codex:luna` gains the reviewer role; the owner put luna:high on the reviewer ladder.
- Global `default_model` becomes `claude:haiku:high` (owner pick).
- A spec test requires `cheap`, `lean` and `smart` on developer, advisor and reviewer.
- Costly models (`sol:high`, `argon:high`, `fable:high`) stay skipped without `--accept-cost`.

## 3. Implementation & Verification Plan

- Edit `spec/agent.yaml`; rerender model-picks blocks; update tests that pin the old chains.
- `harnez agent start --dry-run` for every trait of every leaf role.
- Not in scope: disabling Jules and flash37. The shipped spec has no disable flag (only user
  `models.yaml` `enabled: false`); retiring them is its own ticket.
