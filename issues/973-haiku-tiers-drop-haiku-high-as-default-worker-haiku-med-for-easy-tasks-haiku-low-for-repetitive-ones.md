# 973 — Haiku tiers: drop haiku:high as default worker; haiku:med for easy tasks, haiku:low for repetitive ones

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Docs
**Related**: 956 (made `haiku:high` the default worker; this reverses that part), 969 (Haiku compacts at 80k), 972 (verify before claiming), docs/AutoCompaction.md

---

/goal Every skill, practice doc and `spec/agent.yaml` guide that recommends `haiku:high` instead recommends
`luna:med` as the cheap default developer for well-specified work, `haiku:med` for easy tasks and `haiku:low` for
very simple, repetitive ones, with the reason and source stated, and no Haiku tier promoted as the default developer
worker; or stop and report when blocked on an owner decision.

## 1. Problem & Motivation

Owner feedback, 2026-10-27 (voice transcript; "high quality", "echo" and "cool" read as "Haiku"):

- Haiku 5.5 now compacts at 80k (969, `~/.harnez/config.yaml`), to stay under the 100k price step. High thinking
  effort fills that small budget fast: Haiku thinks a lot, blows up its own context and triggers compaction again and
  again.
- A study the owner saw: Haiku at high effort reaches quality similar to Opus at low effort, but with many more tool
  calls and tokens, so its effective cost was about $3 against $4 for Opus. The cost advantage mostly disappears.
  **Unverified:** no link given; the filer did not find the study. Record the source if it turns up.
- Owner's own experience: `haiku:high` did not do a good job where harnez recommended it.

Requested tiers: `haiku:med` for easy tasks, `haiku:low` for very simple, repetitive tasks. Higher Haiku tiers should
not be promoted.

## 2. Findings: where `haiku:high` is recommended (2026-10-27)

Edit the sources, then sync root copies (`harnez init -d .`, see AGENTS.md "Copyable-Doc Sources"):

- `spec/agent.yaml:120` — Haiku guide: "default worker at haiku:high for well-specified implementation and tests".
- `docs/commands/lean-sprint.md:58`, `docs/commands/epic.md:78` — developer tier `haiku:high`.
- `docs/practices/AgenticLoop.md:305,325-326` and `docs/practices/AgenticLoop.lite.md:41` — Focused Developer
  `luna:high / haiku:high / sol:low`.
- `docs/practices/ModelRoles.md:47` — 2026-10-24 note allowing `haiku:high` in lean-sprint.
- `docs/commands/lean-sprint-v1.md:53` — check whether it names a Haiku tier.
- `docs/studies/*` mention `haiku:high` too; they are history, leave them unchanged.

Open question for the owner: with `haiku:high` dropped, which model is the default developer worker for
well-specified work: `sol:low` alone, or `haiku:med`? The goal above assumes no Haiku tier is the default.

## 3. Implementation & Verification Plan

- Update the files above; keep the 100k price note in the spec guide and add the 80k compaction reason.
- Check tests that assert guide text (for example `TestAgentModelsTableShowsRolesAndUse` in `cmd/harnez/agent_test.go`)
  and update their expectations without weakening the other checks.
- Verify: `grep -rn "haiku:high" docs spec --include=*.md --include=*.yaml` finds only studies and this ticket's
  history; `harnez agent models` shows the new guidance; `make test-q1` passes; `make install`.

## 4. Owner direction and research, 2026-10-27

Owner: prefer Luna medium (`luna:med`) over Haiku as the worker: larger budget before a price step, and in the
owner's view more capable at medium. Two independent web researchers (`luna:med`, `haiku:med`, same prompt) agree:

| | Haiku 5.5 | GPT-6 Luna |
|---|---|---|
| Price in/out per MTok | $0.10 / $0.50 | $0.10 / $0.50 |
| Price step | input > 100k: 5x | input > 272k: 2x input, 1.5x output |
| Window / max output | 1M / 128k | 1.05M (922k input) / 128k |
| Tool use | OSWorld 2.1 72.4%, Terminal-Bench 4.0 39.2% (max effort) | OSWorld 2.1 48.9%, Terminal-Bench 4.0 16.4% (effort not stated) |
| Coding | FrontierCode 1.1 46.4%; SWE-Bench Pro 64.8% (max) | FrontierCode 1.1 42.4%; DeepSWE 66.6% (aggregator, unverified) |
| Research | no comparable benchmark | no comparable benchmark |

Sources: https://platform.claude.com/docs/en/models/haiku-5-5/overview, https://www.anthropic.com/claude-haiku-5-5,
https://developers.openai.com/api/docs/models/gpt-6-luna, https://developers.openai.com/api/docs/pricing.

- No benchmark was found at medium effort for both; published numbers favour Haiku but are not comparable.
- The Haiku-high vs Opus-low study was not found by either researcher.
- Proven Luna advantage: the price step at 272k instead of 100k. "Luna medium is more capable" is unproven.
- Side observation: the `haiku:med` researcher used 211k tokens over one turn (largest single request 36k), against
  37.6k new + 107.5k cached for `luna:med`, on the same prompt. One run each, not a benchmark.

Owner decision (2026-10-27): switch now. The owner started `/lean-sprint 973` after the research, with the
direction "we should rather always use Luna medium". A head-to-head on real tasks stays optional, not a blocker.

## Decision

- Cheap default developer for well-specified work: `luna:med` (owner direction above; proven advantage: price step at
  272k instead of 100k). `sol:low` stays the choice when judgment is needed. `haiku:med` for easy tasks,
  `haiku:low` for very simple, repetitive ones; `haiku:high` is not recommended anywhere.
- The existing line "Luna is for background or mechanical work, not Go development" (lean-sprint and related guides)
  contradicts this default; it is reworded so `luna:med` may take well-specified implementation and tests, while
  judgment-heavy Go work goes to `sol:low`. Reversible docs change; the owner can veto.
- Searcher skipped: section 2 cites exact file:line locations found today; the developer's first turn is plan-only
  and must re-verify them.
- Developer: `luna:med` (owner: no Haiku in this sprint; docs-only edits).

## Milestones

- **M1 (guide text):** edit every location in section 2 plus any `luna`-restriction line it contradicts; sync root
  copies with `harnez init -d .` (revert unrelated rewrites); update tests that assert guide text without weakening
  other checks. Verify per section 3. One commit naming issue 973 M1.
- **M1 delivered (51620455, guide text):** all section 2 locations reworded, root copy synced, both text tests
  updated plus negative `haiku:high` checks. Host review: `go test ./cmd/harnez ./internal/subagent` ok; grep for
  `haiku:high` in docs/spec finds only studies; `harnez agent models` shows the new guidance. The developer quoted
  `make[1]: Leaving directory` instead of a test summary line; not proof of a full pass.
- **M2 (accurate compaction claim) — Pre-Work / Required Refinements:**
  1. "harnez compacts Haiku at 80k" (spec/agent.yaml `claude:haiku` `use`, docs/commands/lean-sprint.md §2, and
     any other place M1 added it) is false for the shipped default: `compact_threshold_tokens` is 200000 for every
     model; 80k exists only in the owner's `~/.harnez/config.yaml` `agent.compact_thresholds`. Reword to say Haiku
     should compact at about 80k via `agent.compact_thresholds` (issue 969, docs/AutoCompaction.md). Update the
     test phrase in `internal/subagent/agentspec_test.go` to match, keeping the 100k/5x checks.
  2. Run `make test-q1` to a file, grep `--- FAIL`, and quote the actual Go test summary (an `ok`/`FAIL` package
     line or the target's final result), not make's directory line. `make install`. One commit naming issue 973 M2.
