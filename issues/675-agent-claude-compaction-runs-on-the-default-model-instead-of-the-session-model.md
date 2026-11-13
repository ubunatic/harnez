# 675 — agent: claude compaction runs on the default model instead of the session model

**Status**: Closed — live claude:haiku:low compaction verified
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [[673-agent-claude-resume-blocked-by-compaction-check-reading-turn-total-as-context-size]]

---

## 1. Problem & Motivation
Since 783a2aa2 (issue 673), `ClaudeDriver.Compact` omits `--model` instead of passing an empty one. The
`/compact` turn then runs on Claude's default model, not the session's model (e.g. a `claude:haiku`
worker compacts with a larger model). That can cost more quota per compaction.

## 2. Technical Specification / Findings
- `Compact` calls `Resume(ctx, id, "/compact", Model{})`; `claudeModelArgs` drops empty flags.
- The session record holds provider/model/tier, so the model is available to the caller.

## 3. Implementation & Verification Plan
- Pass the session's model to `Compact` (same for other drivers if they share the pattern).
- Test: Compact's argv carries the session's `--model`.
- Live check with `claude:haiku:low` only (docs/Canary.md): force a compaction on a haiku worker and
  confirm the verified drop.

## Jules audit (2026-11-10, af9688e0)

Still valid. `ClaudeDriver.Compact` calls `d.Resume(ctx, id, "/compact", Model{})`
(`internal/subagent/claude.go:85`), and `claudeModelArgs` drops an empty model (`claude.go:64-66`), so the
resume carries no `--model`. Report: `reports/jules/675.md` (github#151).

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| implement | jules-impl-675 | 8668048582263024478 | github #157 (closed, code taken) | late PR after the run was counted failed; merged by the host |

## Progress 2026-11-11

Code done from Jules PR GH#157: `Driver.Compact` takes the session `Model`; `ClaudeDriver.Compact` passes it
to `Resume`, and both callers in `cmd/harnez/agent_run.go` (`runResume`, `compactSession`) pass the session's
provider, model and tier. Other drivers accept the parameter and do not use it. The host added
`TestClaudeCompactPassesSessionModel` (`internal/subagent/claude_test.go`), which fails with the old
`Model{}` call; `make test-q1` exit 0; advisor review: no blockers. Left: the live `claude:haiku:low`
compaction check from the plan.

## Live Verification 2026-11-13

- Started a `claude:haiku:low` session (`2f010512-edb1-43df-8c14-4155490223b7`, `model: claude-haiku-5-5`) and
  resumed it with `--model claude-haiku-5-5 --effort low ... "/compact"`.
- Command exited 0 (`"local_command":"compact","subtype":"success"`, hooks `PreCompact`/`PostCompact` ran),
  with `modelUsage` keyed solely by `"claude-haiku-5-5"`.
