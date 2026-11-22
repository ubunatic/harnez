# 1263 — Costly-model gate refuses agy:neon:high in headless orchestrator runs and no approval path reaches the orchestrator

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 1210 (costly gate, closed), 1243 (neon costs, closed), 1262; study `docs/studies/2026-11-21-headless-signal-assistant-one-day-build-story.md` (section 5g)

---

## 1. Problem & Motivation

On 2026-11-21 at 21:49 local, a review was started as `agy:neon:high` and failed at launch, exit 1: `model agy:neon:high is costly (relative cost 219 (codex:luna:low = 1), quota left: unknown); pass --accept-cost to proceed` (run log `sig-rich-review-20261121-214907.log`, 130 bytes; facts file in `docs/studies/.story-1121-facts.md`). The orchestrator did not pass `--accept-cost`. It started `agy:neon:med` reviewers instead, and the study records that substitution as the orchestrator's own decision.

The operator reports the owner wants high-tier reviews. The study records only that the owner said "neon model" with no tier, so the owner's wish for `high` is **unverified**. The gap this ticket covers: a costly-tier choice was made silently by the orchestrator, not by the owner.

## 2. Technical Specification / Findings

- A per-call override exists and is documented: `--accept-cost` on `agent start` and `agent resume` (`docs/HarnezAgentArchitecture.md:133-142`; `docs/commands/lean-sprint.md:66`). So "no per-call override is documented" is inaccurate for harnez. The missing piece is a path the headless orchestrator can use. The study says the orchestrator's spawn command "cannot pass that flag" (`hub spawn`, a separate project, not inspected here).
- `agy:neon:high` is in `costly_models` (`spec/agent.yaml:144-150`). The refusal is `internal/agent/core.go:268-278`.
- `IsCostlyModelFor` reads only the spec (`internal/subagent/agentspec.go:899-906`). A grep found no user-config key for `costly_models`, so persistent owner approval means editing the spec. Unverified beyond the grep.
- Pick fallback skips costly models unless `AcceptCost` is set (`internal/subagent/driver.go:630-661`). That is designed behaviour and is not the reported failure.

## 3. Implementation & Verification Plan

Options, owner decides:

- **A.** Document in the orchestrator contract: pass `--accept-cost` when the owner chose a costly tier; never substitute a tier silently; report the substitution. Doc-only.
- **B.** Let user config approve named costly models, so routine reviewer runs need no per-call flag. Needs a schema entry in `spec/` per `docs/Spec.md`, with no Go default.

Recommendation: A now. Take B only if the owner confirms `neon:high` is a routine reviewer.

Verify: `harnez agent start --dry-run --model agy:neon:high` should print `accept-cost: required` (`cmd/harnez/agent_cost_test.go:240` asserts that text). Confirm the start succeeds with `--accept-cost` in a canary, not in the orchestrator.
