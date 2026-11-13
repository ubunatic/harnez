# 1229 — 993 shipped without the owner decisions of 2026-11-07

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 993

---

## 1. Problem & Motivation

The owner recorded seven decisions for 993 (xhigh and max effort tiers) on 2026-11-07, on the
local branch `issue/993-effort-tiers`. The branch was never pushed. 993 was implemented from
`main` on 2026-11-13 (`feat/jules-batch-c`) and closed without them. The notes reached `main`
only on 2026-11-13, in 993 sections 2b, 4 and 5.

What shipped differs from the decisions in three places. The owner has to say which decisions
still hold; this ticket implements the ones that do.

## 2. Technical Specification / Findings

Checked on `main` at `3511631e`.

| Decision (993 section 4) | Shipped | Source |
| --- | --- | --- |
| 2: `xhigh` and `max` listed in `harnez agent models` by default | Only `claude:haiku:xhigh` is listed; `max` has no row | `harnez agent models`; 993 Progress: "kept `max` opt-in" |
| 3, 4: Sonnet and Opus get the tiers and cost estimates too | Only Haiku has `extra_tiers: [xhigh]`, cost 10 (estimate was 12) | `spec/agent.yaml:217-218`, the only `xhigh` in the file |
| 5: provider scope is claude plus codex, agy and pi | claude and agy accept the tiers; codex and pi are refused | `internal/subagent/driver.go:311` |

Decisions that hold:

- 1: no shipped default uses `xhigh` (no `xhigh` in `spec/agent.yaml` outside the Haiku entry).
- 6: a per-model list decides the tiers; it shipped as `extra_tiers`, not `tiers`.

Unverified:

- Whether `claude:sonnet:xhigh` or `claude:opus:max` start when requested. `ValidateEffortTier`
  reads as if they pass; verify with one dry resolve or a unit test, not a paid turn.
- Whether codex accepts `model_reasoning_effort` values `xhigh` and `max` (993 section 2b).
  `pi --help` lists both for `--thinking` (993 section 2b, help text only).

## 3. Implementation & Verification Plan

- Owner: confirm or drop decisions 2, 3, 4 and 5.
- For each one kept: add the spec entries (`extra_tiers`, `cost_by_tier`) and tests as in 993
  section 5, M3 to M5. Canary codex and pi first if decision 5 is kept (`docs/Canary.md`).
- Safeguard for the cause: `harnez issues close` could warn when a local branch named after the
  ticket (`issue/<n>-*`) holds commits that are not on the default branch. Decide here whether
  that check is worth a ticket of its own.
