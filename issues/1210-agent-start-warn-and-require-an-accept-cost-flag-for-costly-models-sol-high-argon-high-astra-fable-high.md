# 1210 — agent start: warn and require an accept-cost flag for costly models (sol:high, argon:high, astra, fable:high)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Feature
**Related**: 711 (cost matrix), 603 (quota in models/start), 985 (resume cost display), 1208 (start --dry-run), 1019

---

## 1. Problem & Motivation

Owner request (2026-11-10): a session can start `sol:high`, `argon:high`, any `astra` tier or `fable:high`
with no warning, while Codex quota is running low the same day. These models cost far more than the
defaults (`spec/agent.yaml` cost matrix: astra 182/342/384 relative to luna:low = 1). Nothing tells the
caller the cost or the quota left before spending it.

## 2. Goal

/goal `harnez agent start` and `agent resume` refuse a costly model unless the caller passes an
accept-cost flag; the refusal (and the accepted start) prints the model's relative cost and the
provider's remaining quota (5h/weekly as `agent models` knows it, "unknown" when stale). The costly set
comes from `spec/` (with schema), not Go defaults. Tests cover each listed model, a cheap model (no
flag needed), alias/official-name resolution to a costly model, and `--dry-run`. Stop and ask the owner
on flag naming or threshold choices that need their judgment. On a loop or no progress, clear the goal
and write a failure report here.

## 3. Acceptance

- Initial costly set: `codex:sol:high`, `agy:argon:high`, `codex:astra:*`, `claude:fable:high`.
- Without the flag: non-zero exit, message names model, relative cost, quota left and the flag.
- With the flag: one stderr warning line with cost and quota, then a normal start.
- Fallback/pick chains never select a costly model silently without the flag.
- `--dry-run` shows whether the flag would be required.
- `docs/` and the lean-sprint/decide skills mention the flag (helpers stay on cheap models by default).

## 4. Notes / Unverified

- Unverified: whether the set is better defined as a cost threshold (e.g. cost ≥ N) than a list; record
  the choice in a Decision section.
- Check how picks (`--model smart`) map to these models today; a pick resolving to a costly model must
  also need the flag.
