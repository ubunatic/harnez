# 956 — Developer tier defaults: sol:low for judgment, haiku:high as worker, luna:high off Go development (owner feedback)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs
**Related**: 942 (epic: model catalog; facts need sources), 949 (web refresh skill), 954/955/946/948 (slice developer choices)

---

/goal Skills and the shipped model guide reflect the owner's model ranking for development work: `sol:low` for
judgment and orchestration, `haiku:high` as the default worker for well-specified tasks and tests (prompt kept
under 100k tokens), `flash38:med` for routing/preprocessing, `luna:high` not for Go development (log
compaction and similar background work only). Every number that lands in `spec/agent.yaml` carries a source
URL per epic 942 (`web` facts need a URL); figures without a verifiable source are recorded as owner guidance,
not as measured facts. Or stop and report when a source cannot be verified. Circuit breaker: on a loop or no
progress, clear the goal and write a failure report here.

## Owner feedback (2026-10-25, pasted matrix, summary)

| Model | Rating for Go + tool calling | Cost per 1M in/out | Role |
|---|---|---|---|
| sol:low | excellent; Terminal-Bench 58.2%, Intelligence Index 51.8 | $2.00 / $10.00 | brain: orchestration, fixing compiler breaks |
| haiku:high | great; Terminal-Bench 4.0 39.2%, Index 43.0; reliable JSON schema marshalling | $0.10 / $0.50; 5x above 100k prompt ($0.50 / $2.50) | worker: repetitive sub-tasks, tool calls, unit tests |
| flash38:med | good; Coding Index 74.1, Terminal-Bench 4.0 19.7% | $0.38–0.75 / $1.88–3.75 | router: preprocessing, wide-context inputs |
| luna:high | fair; Index ~35; hallucinates in multi-step tool loops | $0.10 / $0.50 | analyst: avoid for Go development |

Cited sources were named only as sites (Anthropic, Command Code, Miraflow AI, OpenRouter, Eigent AI), no URLs.

## Canary result (2026-10-25)

Probed installed Claude Code 2.1.310 with the exact print-mode invocation shape (`claude -p --model haiku --effort low --dangerously-skip-permissions --allowedTools Bash --output-format json`) and child environment `CLAUDE_CODE_AUTO_COMPACT_WINDOW=100000`, `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE=10`. The prompt asked Haiku to generate 50,000 tokens locally through one Bash call. Claude completed without a `compact_boundary`; usage reported `contextWindow: 1000000` and about 52,649 input/cache tokens. At a 100k window with 10% override, compaction should have occurred near 10k, so this probe does not show `-p` honoring the requested settings. Stop item 4; no Claude driver env changes or per-model compact values are planned on this evidence. The probe cost was about $0.006.

## M3 Pre-Work (host review, 2026-10-25)

- Host `make test-q1` after 93100fd5: one failure, `TestAgentModelsTableShowsRolesAndUse`
  (`cmd/harnez/agent_test.go:205`): the luna:low row no longer contains "clear bounded tickets" because M2
  rewrote the luna guide. Update the test to the new guidance; keep its other checks (source, role, cost,
  capability column) and keep or extend the med/high and use_med checks. Do not weaken them.
- Item 4 is parked: the canary was a single `-p` turn, and Claude may compact only between turns. Leave it
  open in the close reason as "not proven for -p"; no driver change.

## Counter-evidence to weigh (this session)

luna:high (codex) delivered 952 (usage staleness, 12 Go files) in one round with a green full run; sol:med
needed 6 milestones on 941 (scope was the cause, see 942 ## Re-cut). One data point each; not a benchmark.

## Scope

1. `docs/commands/lean-sprint.md` (source of the installed skill) and its siblings (`lean-sprint-v1.md`,
   `LeanSprinter.md`, epic/jam skills that name developer tiers): default developer `haiku:high` for
   well-specified work, `sol:low` for judgment; drop "prefer luna first"; mention the 100k Haiku price step.
2. `spec/agent.yaml` model guides (`use`/cost text) for these four models; numbers only with a source URL.
3. Epic 942 table: slice developers 954/955/948 move from luna:high to haiku:high; 946 stays sol:low.
4. Claude auto-compact limit per model (owner, 2026-10-25), like codex's `model_auto_compact_token_limit`
   (`internal/subagent/codex.go:125`): the claude driver sets, in the child env only,
   `CLAUDE_CODE_AUTO_COMPACT_WINDOW` (tokens, 100,000–1,000,000, capped at the model window) and
   `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE` (percent 1–100 of that window; values above the default threshold are
   ignored). Haiku example: window 100000 + pct 85 → compacts near 85k, below the 100k price step. Values in
   `spec/agent.yaml` per model, not Go defaults. Sources: https://code.claude.com/docs/en/env-vars#variables,
   https://code.claude.com/docs/en/model-config#set-the-auto-compact-window. Docs do not say whether `-p`
   honours them: canary first (installed 2.1.310), proving a compaction at the expected size; test like
   `TestCodexExecArgsUseConfiguredAutoCompactLimit`.
5. Not in scope: a Go context limiter for the 100k threshold (the paste's closing offer); file separately if
   wanted.
