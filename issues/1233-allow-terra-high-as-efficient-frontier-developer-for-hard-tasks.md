# 1233 — Allow terra:high as efficient-frontier developer for hard tasks

**Status**: Closed — bdbdd7e0 + 1fed6ab8: terra:high in roles.developer.models.smart and codex:terra.use guidance; pickdocs/docs synced; make test-q1 green; live dry-run + --picks verified
**Closed-By**: session 801d927d-19ac-46f0-aece-6142997b71d9
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 974, 983, 1225

---

## 1. Problem & Motivation
`roles.developer.models` in `spec/agent.yaml` maps `lean: [terra:low]`, `std: [terra:med, opus:low]`, and `smart: [sol:med, opus:med]`, skipping `terra:high` entirely.

At relative cost `36` (and Codex subscription multiplier `0.19`), `codex:terra:high` sits between `terra:med` (`24`) and `sol:med` (`47`) and proved strong on complex multi-step work (e.g. issue 1225). It should be available as an efficient-frontier developer option for hard tasks in `spec/agent.yaml` (and its rating/guidance in `spec/model_scores.yaml` and `spec/agent.yaml` aligned accordingly).

## 2. Technical Specification / Findings
- In `spec/agent.yaml`:
  - Include `terra:high` in `roles.developer.models.smart` (e.g. `smart: [terra:high, sol:med, opus:med]` or alongside `sol:med`) and update `codex:terra.use` guidance so `terra:high` is recommended as an efficient-frontier developer for hard tasks before escalating to `sol:med`/`sol:high`.
- In `spec/model_scores.yaml`:
  - Review `codex:terra` capability ratings (`low: 2.5, med: 3.0, high: 3.5` vs `codex:sol` `low: 3.5, med: 4.0, high: 4.5`) and update if needed so `terra:high` reflects its frontier position on hard tasks.
- Update affected golden files (`cmd/harnez/testdata/agent-models-chart-80cols.golden`, `internal/subagent/testdata/known-model-specs.golden`) and tests in `internal/subagent/`.

## 3. Implementation & Verification Plan
- Update `spec/agent.yaml` (and `spec/model_scores.yaml` if ratings change) and regenerate/update golden fixtures.
- Verify `harnez agent models --picks` and `harnez agent models --chart`, then run `make test-q1` and `make install`.

## Decision
- Add `terra:high` to `roles.developer.models.smart` ahead of `sol:med` (`[terra:high, sol:med, opus:med]`) and update `codex:terra.use` guidance so `terra:high` serves as the efficient-frontier Codex developer for hard tasks before escalating to `sol:med`/`sol:high`.
- Keep `spec/model_scores.yaml` star ratings (`codex:terra.high: 3.5`) unchanged: ratings are owner-estimated benchmarks, and changing `terra:high` without measured evidence is reversible later without altering chart placement vs `claude:sonnet:low` (cost 15, ★4.0).

## Pre-Work (Scout findings)
- `spec/agent.yaml:412`: `roles.developer.models.smart` — prepend `terra:high` (`smart: [terra:high, sol:med, opus:med]`).
- `spec/agent.yaml:185-196`: `models.codex:terra.use` — update guidance to note `terra:high` as efficient-frontier developer for hard tasks before escalating to Sol.
- `internal/subagent/agentspec_test.go:96`: check/update assertion on `codex:terra` guidance if it pins exact `use` text.
- `cmd/harnez/agent_picks_test.go:59`: check/update assertion on `developer` `smart` chain length and first resolved pick (`terra:high`).
- `internal/subagent/testdata/known-model-specs.golden:39` & `internal/subagent/driver_test.go:150`: already list `codex:terra:high` (verify passes).
- `cmd/harnez/agent_chart_test.go:261` & `cmd/harnez/testdata/agent-models-chart-80cols.golden`: verify passes untouched (if chart changes, do not touch without reason).
- Tests to run: `go test -race -count=1 ./internal/subagent ./cmd/harnez` (remember Quota-1 single test run if needed, or run targeted `go test` on those two packages before final commit).

## M2 Pre-Work (`make test-q1` failure)
- `internal/pickdocs/pickdocs_test.go:69-84`: `TestSourceBlocksCurrent` failed because the embedded `<!-- harnez:model-picks:begin -->...<!-- harnez:model-picks:end -->` tables are stale in:
  - `docs/practices/AgenticLoop.md`
  - `docs/practices/AgenticLoop.lite.md`
  - `docs/practices/ModelRoles.md`
  - `docs/commands/lean-sprint.md`
  - `docs/commands/lean-sprint-v1.md`
  - `docs/commands/epic.md`
  - `docs/commands/HarnezStatus.md`
- Also check `AGENTS.md` Rules and Copyable-Doc Sources: after updating copyable doc sources in `docs/practices/`, run `make install` and `harnez init -d .` (and `git checkout --` any unrelated file `init` rewrote) so `TestRootDocCopiesMatchSources` passes (`docs/AgenticLoop.md`, `docs/ModelRoles.md`, and any synced skill/command copies).
- Verify with `go test ./internal/pickdocs ./internal/claude ./internal/subagent` (or `make test-q1` if available).

**/goal** Allow `terra:high` as an efficient-frontier developer for hard tasks in `spec/agent.yaml` (and model scores/goldens), verified by `harnez agent models --picks` and tests, or stop and report when blocked on a user decision or denied permission.
