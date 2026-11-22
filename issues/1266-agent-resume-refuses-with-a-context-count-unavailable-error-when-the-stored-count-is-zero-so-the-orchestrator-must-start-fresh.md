# 1266 — agent resume refuses with a context-count-unavailable error when the stored count is zero, so the orchestrator must start fresh

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 644, 673, 594 (closed), 1008, 1264

---

## 1. Problem & Motivation

The hub's orchestrator resumes workers with `harnez agent resume`. On a `claude:opus:high` writer that had hit a quota failure, resume was refused with `refusing to send resume prompt: current context token count is unavailable; refusing to dispatch prompt`. The orchestrator could only start a fresh session, and it lost its context. The refusal names "unavailable" context, but the code path does not distinguish an unknown measurement from an unset field (see section 2).

Source: the hub-run report (orchestrator, 2026-11-22). The refused session's stored record was not inspected.

## 2. Technical Specification / Findings

- `internal/subagent/compact.go:92-101` (`EnsureContextUnderThreshold`): a negative count is unknown and skips compaction; a count of **zero** returns `current context token count is unavailable; refusing to dispatch prompt`. The comment says zero "remains unavailable and blocks dispatch".
- `cmd/harnez/agent_run.go:1086-1094`: that check runs only when `contextTokens >= 0 && (compactMode == "manual" || interactive)`. Its error is wrapped as `refusing to send resume prompt`.
- `spec/agent.yaml:78`: `claude: manual`, so Claude workers hit this check on every resume.
- `internal/subagent/session.go` (`LastContextTokens`): returns -1 when the stored value is negative; returns 0 when the stored value is 0 and no turn record carries input tokens.
- `internal/subagent/claude.go:237`: a Claude turn with no usage reports -1, not 0, and that passes the check. Zero-usage synthetic events are skipped (`claude.go:199-204`). A Claude turn therefore does not produce 0 by itself.
- `internal/subagent/session.go:236-237` (`marshalStoredSession`) keeps -1 on disk and drops 0 (`omitempty`). A stored record with no `context_tokens` reloads as 0, which is what triggers the refusal. This means 0 means "never recorded", not "measured as zero".
- Unverified: which stored state the hub's refused session held. Check the session's `context_tokens` field in the registry JSON; a missing field means the refusal came from this path.

## 3. Implementation & Verification Plan

Owner decision first:

1. Treat a never-recorded count (0) like unknown (-1): skip compaction, dispatch, and print a warning. The code already defines negative values this way.
2. Keep the refusal, but name the cause ("no context count recorded for this session") and the next command the host should run.

Option 1 is the lean choice; it is a recommendation, not a decision. Canary before the fix: a unit case in the resume table (`cmd/harnez/agent_resume_test.go`, near line 575) with a stored session that has no `context_tokens` and a claude driver. Expect the refusal today. After the fix, run `make test-q1` for the quota-wrapped run.
