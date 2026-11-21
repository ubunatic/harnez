# 992 — Per-provider plan preference: own, free or shared plan limits which roles a provider gets

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 942, 941, 983, 603, 991

---

/goal Each user can declare per provider (claude, codex, agy) what kind of plan they use. Model
picks and `harnez agent` follow it: a shared plan gets only one-shot review/report work, never
mechanical or looping dev work, and a free plan is used sparingly. Visible in
`harnez agent models --picks`. Verify with the two use cases below, or stop and report when
blocked on a user decision or denied permission. Circuit breaker: on a loop, a hard
architectural or OS blocker, or no progress, clear the goal and write a failure report here
(what was tried, why it failed, resume point).

## Problem (owner, 2026-10-28)

The picks in `spec/agent.yaml` assume one user with their own paid plans everywhere. Two real
setups on the same codebase differ:

- **Use case A (Uwe):** agy, claude and codex, all on their own paid (Pro) plans. The current
  defaults fit.
- **Use case B (Alfred):** own claude, free codex (tiny quota), and agy on Uwe's shared Google
  AI Pro plan. The defaults would send Alfred's dev loops to codex (free) and could send
  mechanical work to agy, which uses up Uwe's shared quota.

## Wanted

A basic per-provider preference in the user policy (`$XDG_CONFIG_HOME/harnez/models.yaml`,
from 941), e.g. `plan: own | free | shared` per provider:

- `own`: no restriction (today's behaviour).
- `free`: low priority in every chain; skip it once quota is low or unknown (needs 991's Codex
  quota read).
- `shared`: allowed only for review and advisor/report roles, never for dev or mechanical
  work. Use it one-shot: the caller sends all snippets, docs and files and asks for one answer
  or one report file, with no exploring or looping. `harnez agent` refuses or reroutes a
  `--role dev` pick that resolves to a shared provider and says why.

`--picks` shows the plan per provider and the chain after filtering. Keep this small: one
key per provider plus role filtering; the catalog and access work stays in 942.

## Open questions

- Should the shared one-shot mode be enforced (e.g. no tool use, single turn) or only steer
  prompts/skills? Suggestion: steer first, enforce later.
- Does `shared` also need a budget cap per day/week, or is role filtering enough for v1?

## Handoff from 991 (2026-10-29)

991 Item 4 (picks fallback) was not implemented there and moved here. Plan:
`docs/studies/2026-10-28-argon-991-refinement-draft.md` Item 4. In short, in
`ProviderQuotaAvailabilitiesFromStore` (`internal/usage/statecache.go`): treat a window at
`UsedFraction >= 1` with a nil `ResetAt` as exhausted when fresh; and when a provider has no
quota windows but the latest snapshot says `Installed && !Authenticated`, mark it unavailable so
`cachedModelPicks`/`selectStartSelection` fall back. A missing store or no snapshot must stay
`unknown` (keeps `agent-picks.golden`). Re-verify the line numbers; they are a day old.

## Owner decision: plan-gated `luna:high` (2026-11-20)

The 2026-11-20 picks (`spec/agent.yaml`, `docs/Models.md`) follow each OpenAI pick above
`luna:med` with `luna:high`, e.g. developer `lean: [terra:low, luna:high, haiku:high, neon:low]`.
Without an OpenAI subscription only Luna is usable, so `luna:high` is the OpenAI fallback there.
Today the chain falls through for any skip reason (quota exhausted, user policy, costly gate),
so a subscription account can land on `luna:high`. Wanted:

- **Subscription plan known** (`own`, or a detected paid plan): never pick `luna:high`; skip it
  with a visible reason ("only for accounts without an OpenAI subscription").
- **Free plan known**: `luna:high` keeps its place right after the OpenAI pick.
- **Plan unknown**: try every other model in the chain first (`sol:low`, the other vendors);
  `luna:high` moves to the end of the chain as the last resort.

Plan source: the declared `plan` from the user policy above wins. Otherwise detect it from the
Codex login: `internal/usage/codex.go:344` already reads the `chatgpt_plan_type` claim for
`harnez usage`. Canary first: confirm the value a no-subscription account reports (expected
`free`) and a paid one (`plus`, `pro`, ...) before mapping values.

Which models are plan-gated belongs in `spec/agent.yaml` with a schema (`docs/Spec.md`), e.g. a
per-provider list of no-subscription-only model specs, not in Go. `--picks` shows the plan per
provider and the filtered or reordered chain. Tests: subscription plan with Terra and Sol
exhausted never resolves `luna:high`; free plan picks it in place; unknown plan resolves it only
after every other candidate is skipped.
