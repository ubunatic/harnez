# 1204 — Add Claude Fable 5.2 as a harnez agent model

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 755 (added Haiku 5.5, same pattern), 1203 (official model names), 983 (model picks)

---

## 1. Problem & Motivation

Owner request (2026-11-10): support Claude Fable 5.2 as a harnez model. `harnez agent models`
lists Claude Haiku, Sonnet and Opus but no Fable entry, and `spec/` has no mention of Fable,
so `harnez agent start --model fable` cannot dispatch it.

## 2. Goal

/goal `harnez agent start --model claude:fable[:<tier>]` dispatches Claude Fable 5.2
(official ID `claude-fable-5-2`) and `harnez agent models` lists it, with every value coming
from `spec/agent.yaml`; stop and report when a needed value (price, effort support, roles) has
no source and the choice is the owner's. On a loop or no progress, clear the goal and write a
failure report here.

## 3. Acceptance

- A `claude:fable` entry in `spec/agent.yaml` (schema-valid) modeled on the 755 Haiku entry.
- `harnez agent models` shows `claude:fable:low|med|high`; a dry run or resolver test resolves
  `--model fable` to it.
- No live call to Fable (owner, 2026-11-10). Verify by resolver/dry-run tests only.
- Cost values are estimates, marked as such in the spec entry's comment or source field, with
  the basis stated (e.g. relative to the existing Opus/Sonnet cost values and any published price).

## Decision

- 2026-11-10 owner: do not call the model; add it and estimate the costs.
- 2026-11-10 host: no published Fable price, effort or context source was found (searcher
  s1204). Estimate cost = Opus value (20) and copy Opus's effort/compaction handling; mark `eff`
  and `skills` unknown (`?`), and say "estimate, unverified" in `use`. Reason: same vendor
  flagship tier; reversible once a price is measured.

- 2026-11-10 host, superseding the estimate above: web search (w1204, haiku:med) found the
  official price on https://platform.claude.com/docs/en/about-claude/pricing: Fable 5.2 $8 in /
  $40 out per MTok, exactly 2x Opus 5.5 ($4/$20). Set `cost: 40` (2x Opus's 20) and replace the
  "estimate, unverified" note with that source. Context 1M tokens, effort low..max
  (https://platform.claude.com/docs/en/models/fable-5-2/overview).

## M1 Pre-Work / Required Refinements (after the first failed make test-q1)

1. Re-run covers the already-applied `skills` schema fix (it caused most of the schema failures:
   TestKnownErrorChecksMatchAgentSchema, TestPurposeAliasSchema, TestAgentSpecLmcoder..., TestLegacy...).
2. TestBenchSetupRunResults, TestStoreRunTasksAndSummaries, TestModelPickerMarkersAndRenderAt80Cols
   assumed the first Claude model or picker layout. Updating their expectations is authorized; keep
   each assertion as strict as before (same exact-match form, no Contains for Equal). If a test
   reads "first Claude model" as a default, prefer keeping that default (e.g. entry order) over
   rewriting the test, and say which you chose.
3. Apply the cost 40 decision above; docs/Models.md shows 40 with the pricing URL.

## Searcher findings (s1204, codex:luna:low)

- Template: `claude:opus` in `spec/agent.yaml:201-211`; Haiku at 177-189 shows `guidance_source_url`.
- Driver passes `--model <name> --effort <tier>` (`internal/subagent/claude.go:64`); `claude --help`
  accepts the alias `fable` and full names. Use `name: claude-fable-5-2`.
- Goldens: `internal/subagent/testdata/known-model-specs.golden`, maybe
  `cmd/harnez/testdata/agent-picks.golden` (picks unchanged, so likely untouched).
- 755 also touched resolution asserts in `cmd/harnez/agent_test.go`,
  `internal/bench/bench_test.go`, `internal/subagent/driver_test.go`; docs table `docs/Models.md:66`.

## 4. Notes / Unverified

- The ID `claude-fable-5-2` comes from Claude Code's model list; whether the Claude Code CLI
  takes `fable` as a `--model` alias is unverified (check `claude --help` or one live run).
- Cost, effort support, compaction threshold, skills and roles are unknown; mark them `?`
  until measured, or ask the owner. Whether agy also offers Fable is unknown.
- Re-check `spec/agent.yaml` and recent commits before starting; 1203 may change name resolution.
