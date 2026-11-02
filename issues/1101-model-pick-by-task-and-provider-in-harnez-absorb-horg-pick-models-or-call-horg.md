# 1101 — Model pick by task and provider in harnez: absorb horg pick/models or call horg

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 1100, 711, 974; codeberg.org/harnez/horg#7, horg#8, horg#9

---

/goal `harnez agent start` can take a model by task and provider ("a scout with an OpenAI model") and skip
models the account cannot run now. Pick one of two routes first, as an owner decision: (A) absorb horg's
Artificial Analysis client, `horg models` and `horg pick` into harnez, or (B) call the `horg pick` binary.
Done when the chosen route works live and `make test-q1` is green. Stop and report when blocked on a user
decision or denied permission. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and
write a failure report here.

## 1. Problem & Motivation

Picking a model by vendor and account state is manual in harnez today. horg (codeberg.org/harnez/horg) ships it:

- `horg models` (`internal/aa`): Artificial Analysis free-tier client (100 requests/day, key in
  `~/.harnez/external/artificialanalysis/config.json`), 12 h raw cache, engineering index, cost per task,
  subscription factor (`spec/subscriptions.yaml`).
- `horg pick` (`internal/pick`, horg `82786a5`, `8d1e693`): filters `harnez agent models --json` and
  `--picks --json` by model vendor, skips high tiers, falls back to `spec/picks.yaml` ladders, and skips
  models blocked by login, plan (`plans:`) or quota (`harnez usage --json --offline`, `--min-quota`).

## 2. Options

- **A, absorb:** one binary, no horg dependency, picks can use AA scores directly. Costs about 1000 lines
  plus spec files, and an API-key client inside harnez.
- **B, CLI call:** e.g. `harnez agent start --pick scout@openai` runs `horg pick`; small change, but harnez
  depends on an optional binary and horg already calls harnez (a cycle of processes, not of code).
- Open: role names follow 1100 (`scout`, `consult`); the plan rule (Codex Free rejects `sol`) may belong in
  harnez's own model list either way.

## 3. Verification Plan

Re-check horg and harnez live code first. Live: a provider pick resolves and starts a worker; a blocked model
is skipped with a reason; `make test-q1` green.
