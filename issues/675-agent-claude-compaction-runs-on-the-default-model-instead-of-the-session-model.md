# 675 — agent: claude compaction runs on the default model instead of the session model

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
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
| implement | jules-impl-675 | 8668048582263024478 | none | failed: asked for plan approval twice (replies: 2), no PR |

Lead from its plan (unverified): add a `Model` parameter to `Compact` on the `Driver` interface and every
driver, pass it to `d.Resume` in `ClaudeDriver.Compact`, and pass the session model from the callers in
`cmd/harnez/agent_run.go`.
