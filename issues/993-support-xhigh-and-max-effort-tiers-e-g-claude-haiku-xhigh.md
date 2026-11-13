# 993 — Support xhigh and max effort tiers (e.g. claude:haiku:xhigh)

**Status**: Closed — resolved on feat/jules-batch-c (unit, schema, catalog, driver, and CLI chart/listing tests pass; claude & agy --help verified; live claude/agy network turn skipped to preserve weekly quota)
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

## 2b. Provider addendum (host, 2026-11-07)

Help text only, no flag run: `agy --help` lists `--effort low|medium|high|xhigh|max`, so the
"unknown" above is answered for agy. `pi --help` lists `--thinking off, minimal, low, medium,
high, xhigh, max`. `codex exec --help` has **no** effort flag; harnez sets `model_reasoning_effort`
with `-c`, and which values it accepts is still unverified. Acceptance is assumed, not canaried
(owner decision 7); the per-model `tiers` field below is the lever to disable a broken combo later.

## 3. Plan

1. Canary: `claude --effort xhigh` and `--effort max` with Haiku 5.5 in harnez's headless mode;
   record whether each is accepted.
2. Extend the tier enum (schemas, spec patterns, tier aliases) with `xhigh` and `max`; map them to
   `--effort` for claude; reject them for providers without support, with a clear error.
3. Add `claude:haiku:xhigh` (and the other claude models where accepted) to the spec with
   cost estimates; keep `max` opt-in.
4. Test: tier parsing, the claude effort mapping, and a `models.yaml` chain containing
   `haiku:xhigh`; `make test-q1` green; `make install`; live `harnez agent models` check.

## 4. Decision (owner + advisor plan, 2026-11-07)

Owner decisions, recorded before coding:

1. **xhigh is never a shipped default.** It is allowed when requested and reachable through a
   `models.yaml` chain, but shipped role picks stay on `low`/`med`/`high`.
2. **`xhigh` and `max` are allowed by default**, i.e. listed in `harnez agent models` and
   accepted by parsing/policy; they are not gated behind an opt-in switch.
3. **No sourced xhigh/max cost data exists** in the repo (checked `reports/`, `docs/studies/`,
   `docs/Models.md`, `spec/agent.yaml`, 517/969/973). Current shipped anchors are relative costs
   anchored at `codex:luna:low = 1`: Haiku 5.5 `low 4 / med 4 / high 7` (`spec/agent.yaml:181-182`),
   Sonnet `low 15` (`:194`), Opus `low 20` (`:205`). The Artificial Analysis figures in §1 are
   quality scores (high 30, max 36), not costs.
   **Estimate used (owner will replace after measurement):** per-tier multiplier `1.75` (the
   observed Haiku med->high ratio), `low = med`, `xhigh = round(high x 1.75)`, `max = round(xhigh x 1.75)`:
   Haiku `xhigh 12 / max 21`; Sonnet `med 15 / high 26 / xhigh 46 / max 80`; Opus `med 20 / high 35 / xhigh 61 / max 107`.
   These only rank fallback order, so monotonic increase matters more than the absolute number.
   Each entry gets a spec comment marking it as an estimate to be replaced.
4. **Sonnet and Opus get the tiers too**, not only Haiku.
5. **Provider scope: claude plus codex, agy and pi.** agy sends `--effort`, pi sends `--thinking`,
   codex keeps `-c model_reasoning_effort=`. `off`/`minimal` (pi) stay out of scope.
6. **Per-model `tiers` list wins over one global enum** — providers and models differ (pi has extra
   levels, codex acceptance unknown), and the field is how a broken combo gets disabled. Default
   stays `[low, med, high]` so existing entries keep today's behaviour.
7. **No canary runs.** Acceptance is assumed for this pass; broken combinations get flagged and
   disabled later once observed.

## 5. Milestones (refined by the architect plan, 2026-11-07)

- **M1 Spec + schema:** one tier enum in `spec/schemas/agent.schema.json` (`$defs/tier`) used at the
  model tier and `cost_by_tier` places; add optional `cost_by_tier.xhigh/.max`; new per-model
  `tiers` list defaulting to `[low, med, high]`; widen `spec/schemas/model-policy.schema.json`
  regex to `(:(low|med|high|xhigh|max))?$` and its tier enum; declare the tier list once in
  `spec/agent.yaml` so Go never redefines it.
- **M2 Parsing + mapping:** `AllTiers` read from the spec plus `Model.SupportedTiers()` replacing the
  hard-coded lists in `internal/subagent/driver.go` (`isTier`, the spec tier check, the `models`
  listing, `modelEnabledAtAnyTier`) and `modelcatalog/catalog.go` (`policyIDTier`, tier fan-out);
  collapse the five effort mappers (`claudeEffort`, `codexEffort`, `agyEffort`, `piThinkingLevel`,
  `providerEffort`) into one `effortLevel(tier)` keeping `med` -> `medium`. Distinct errors:
  unknown tier stays `unknown model tier in requested spec %q`; unsupported-but-valid tier gets
  `model %q does not support effort tier %q (supported: ...)`.
- **M3 Model entries + costs:** `tiers` plus `cost_by_tier` estimates from §4 item 3 for Haiku,
  Sonnet, Opus (and codex/agy/pi where the tier is plausible), `compact_thresholds` for each new
  tier key (`spec/agent.yaml:14-16`, validated in `agentspec.go:250`).
- **M4 Help + listing + fixtures:** listing follows `SupportedTiers()`; regenerate
  `internal/subagent/testdata/known-model-specs.golden` and re-grep other tier listings.
- **M5 Tests (table-driven, no sleeps):** resolve `claude:haiku:xhigh`; reject a tier a model does
  not support; `bogus` tier still hits the unknown-tier error; `effortLevel` mapping plus argv
  showing `--effort xhigh`; a `models.yaml` chain containing `haiku:xhigh` validates and resolves;
  catalog `IsEnabled("claude","haiku","xhigh")`. Gate `make test-q1`, then `make install`.
- **M6 Docs:** edit the copyable **sources** in `docs/practices/` and `docs/lang/` (not root copies),
  then `make install` -> `harnez init -d .` -> `git checkout --` unrelated rewrites; add a note that
  higher tiers fill Haiku's 80k compaction budget faster (see 969/973).
- **M7 Live check:** `harnez agent models` shows the xhigh/max rows; one trivial
  `claude:haiku:xhigh` turn proves `--effort xhigh` reached the argv; one rejection run for an
  unsupported model/tier; close 993.

## Audit 2026-11-08 (Jules)

Jules audit (GitHub PR 146, `reports/jules/993.md`, checked against d8c61da; 5/5 references hold): still valid. Tiers are closed to `low|med|high` in `spec/schemas/agent.schema.json:14,240` (ticket said 189; it moved), `spec/schemas/model-policy.schema.json:28,45` and `isTier` (`internal/subagent/driver.go:897`); `claude.go:78` maps `med` to `medium`. The `claude --help` claim stays not verified. Next: extend the enums, pattern and Go tier lists with `xhigh`/`max`, and decide how non-Claude providers treat them.

Re-audit 2026-11-11 (github#184, checked against 5882420e; 6/6 references hold): same verdict. The second
`agent.schema.json` tier enum is now at line 275 and `isTier` at `driver.go:1039`. The earlier report is kept.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-993 | 10749928295299853226 | github #184 (closed) | done: still valid, 6/6 refs hold; repeat |

## Progress 2026-11-13

- Canary (`claude --help`, `agy --help`): both confirm `--effort` accepts `(low, medium, high, xhigh, max)`.
- Implemented `xhigh` and `max` tiers in `spec/schemas/agent.schema.json`, `spec/schemas/model-policy.schema.json`, `modelcatalog` (`ExplicitlyEnabled`, `LegacyGuide.ExtraTiers`), and `internal/subagent` (`SupportsEffortTier`, `ValidateEffortTier`, `KnownModelEntriesE`, `resolveExistingModelIn`, `resolvePiModelWithError`, driver & interactive guards).
- Added `claude:haiku:xhigh` (`cost_by_tier.xhigh: 10`, `extra_tiers: [xhigh]`, `spec/model_scores.yaml` `xhigh: 3.5`), kept `max` opt-in, rejected `xhigh`/`max` on unsupported providers (`codex`, `pi`, etc.) and `effort: false` models.
- Updated `TestKnownModelSpecsGoldenWithoutPolicy`, `agent-models-chart-80cols.golden`, `modelcatalog` schema/policy tests, and added `TestXHighAndMaxEffortTiers`.
