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

## Counter-evidence to weigh (this session)

luna:high (codex) delivered 952 (usage staleness, 12 Go files) in one round with a green full run; sol:med
needed 6 milestones on 941 (scope was the cause, see 942 ## Re-cut). One data point each; not a benchmark.

## Scope

1. `docs/commands/lean-sprint.md` (source of the installed skill) and its siblings (`lean-sprint-v1.md`,
   `LeanSprinter.md`, epic/jam skills that name developer tiers): default developer `haiku:high` for
   well-specified work, `sol:low` for judgment; drop "prefer luna first"; mention the 100k Haiku price step.
2. `spec/agent.yaml` model guides (`use`/cost text) for these four models; numbers only with a source URL.
3. Epic 942 table: slice developers 954/955/948 move from luna:high to haiku:high; 946 stays sol:low.
4. Not in scope: a Go context limiter for the 100k threshold (the paste's closing offer); file separately if
   wanted.
