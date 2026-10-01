# 675 — agent: claude compaction runs on the default model instead of the session model

**Status**: Open
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
