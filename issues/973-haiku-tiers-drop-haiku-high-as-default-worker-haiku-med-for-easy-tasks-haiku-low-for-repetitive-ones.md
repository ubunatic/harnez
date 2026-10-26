# 973 — Haiku tiers: drop haiku:high as default worker; haiku:med for easy tasks, haiku:low for repetitive ones

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Docs
**Related**: 956 (made `haiku:high` the default worker; this reverses that part), 969 (Haiku compacts at 80k), 972 (verify before claiming), docs/AutoCompaction.md

---

/goal Every skill, practice doc and `spec/agent.yaml` guide that recommends `haiku:high` instead recommends
`haiku:med` for easy, well-specified tasks and `haiku:low` for very simple, repetitive ones, with the reason and
source stated, and no Haiku tier promoted as the default developer worker; or stop and report when blocked on an
owner decision.

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
