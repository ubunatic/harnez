# 993 — Support xhigh and max effort tiers (e.g. claude:haiku:xhigh)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [[517-check-claude-effort-support-in-harnez-agent]], [[973-haiku-tiers-drop-haiku-high-as-default-worker-haiku-med-for-easy-tasks-haiku-low-for-repetitive-ones]], [[969-per-model-compaction-limit-and-percent-with-re-compact-and-post-compact-fill-guards]], [[992-per-provider-plan-preference-own-free-or-shared-plan-limits-which-roles-a-provider-gets]]

---

/goal `harnez agent --model claude:haiku:xhigh` (and `:max`) starts a session that passes the
matching `--effort` to the Claude CLI, the tiers appear in `harnez agent models`, and user
`models.yaml` chains accept them — or stop and report when blocked on a user decision or denied
permission. Circuit breaker: on a loop, a hard blocker, or no progress, clear the goal and write
a failure report here (what was tried, why it failed, resume point).

## 1. Problem & Motivation

harnez tiers are `low | med | high` only (`spec/schemas/agent.schema.json:14,189`,
`spec/schemas/model-policy.schema.json` tier and spec patterns). The installed Claude CLI accepts
`--effort low|medium|high|xhigh|max`. Owner (Alfred, 2026-10-28): Haiku 5.5 xhigh is nearly as
good as max at about half the price, and is the planned rung between `haiku:high` and
`sonnet:high` in the developer ladder (`haiku:high -> haiku:xhigh -> sonnet:high -> opus:med`),
used by forgewerk epic 001 as its job model. Artificial Analysis (index v5.0): Haiku 5.5 high 30,
max 36; xhigh not shown.

## 2. Findings

- `claude --help`: `--effort <level>` lists `low, medium, high, xhigh, max`.
- 517 maps tiers to `--effort` (med -> medium) for explicit tiers only.
- Haiku compacts at 80k by default (969); higher effort fills that budget faster (973 note).
- Other providers: whether codex/agy offer comparable tiers is unknown; scope to claude unless
  a canary shows support.

## 3. Plan

1. Canary: `claude --effort xhigh` and `--effort max` with Haiku 5.5 in harnez's headless mode;
   record whether each is accepted.
2. Extend the tier enum (schemas, spec patterns, tier aliases) with `xhigh` and `max`; map them to
   `--effort` for claude; reject them for providers without support, with a clear error.
3. Add `claude:haiku:xhigh` (and the other claude models where accepted) to the spec with
   cost estimates; keep `max` opt-in.
4. Test: tier parsing, the claude effort mapping, and a `models.yaml` chain containing
   `haiku:xhigh`; `make test-q1` green; `make install`; live `harnez agent models` check.
