# 1266 — agent resume refuses with a context-count-unavailable error when the stored count is zero, so the orchestrator must start fresh

**Status**: Closed — M2 (80d0efda) updated EnsureContextUnderThreshold (<= 0) and agent_run.go to warn and dispatch resume when context_tokens is unset (0) while keeping VerifyCompaction strict; HTO=0 make test exit 0
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

## 3. Decision, Goal & Verification Plan

/goal Allow `harnez agent resume` to proceed with a clear warning when a stored session has no recorded context token count (`context_tokens == 0`), matching the existing unknown (`-1`) contract, and verify with unit/regression tests; or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural blocker, or no progress, clear the goal and record a failure report with resume point.

## Decision

- Selected Option 1 (autonomous `/decide`, 2026-11-24): treat a never-recorded count (`0`, produced when `omitempty` omits an unset `context_tokens` field) like unknown (`-1`) — skip compaction, print a warning, and dispatch the resume prompt rather than destroying session continuity.

Canary before the fix: a unit case in the resume table (`cmd/harnez/agent_resume_test.go`, near line 575) with a stored session that has no `context_tokens` and a claude driver. Expect the refusal today. After the fix, run `make test-q1` for the quota-wrapped run.

## 4. Architect Refinement (`argon:med`, 2026-11-24) — M2

- **Exact fix**:
  - `internal/subagent/compact.go:92-104` (`EnsureContextUnderThreshold`): treat `contextTokens <= 0` (both `-1` unknown and `0` never-recorded) as skip compaction (`return false, nil`). Keep `VerifyCompaction` (`after <= 0 || after >= before`) strictly enforced.
  - `cmd/harnez/agent_run.go:1089-1095`: when `compactMode == "manual" || interactive`, if `contextTokens == 0`, emit `fmt.Fprintf(cmd.ErrOrStderr(), "harnez: warning: session %q has no recorded context token count; skipping compaction check\n", sess.Name)` and proceed; if `contextTokens > 0`, call `subagent.EnsureContextUnderThreshold`.
- **Write set**: `internal/subagent/compact.go`, `internal/subagent/compact_test.go`, `cmd/harnez/agent_run.go`, `cmd/harnez/agent_resume_test.go`.
- **Tests**:
  - Canary + regression in `cmd/harnez/agent_resume_test.go`: `TestClaudeResumeWithUnsetZeroContextTokensWarnsAndDispatches` (`ContextTokens: 0` omitted by `omitempty` -> succeeds, `compactCalls == 0`, `resumeCalls == 1`, stderr contains `"no recorded context token count; skipping compaction check"`).
  - Unit case in `internal/subagent/compact_test.go`: `EnsureContextUnderThreshold(0, 200, ...)` returns `false, nil` without invoking `compact`.

## 5. Pre-Work / Required Refinements (from M1 #1275 Review)

Execute these M1 fixes first (in the M2 commit or a clean M1-fixup commit before M2):
1. `[MED]` `internal/exec/quota1_command.go`: remove `buildvcs` from `goTestValueFlags` (boolean in `go test`).
2. `[MED]` `internal/exec/quota1_command.go`: recognize `-test.short`, `-test.run`, `-test.bench`, `-test.fuzz` aliases alongside `-short`, `-run`, `-bench`, `-fuzz`.
3. `[MED]` `.harnez/rules/Quota.md`: remove the stale duplicate `Single-Test Boundary` / `Code Modification Required` lines (3-4) and keep the single clean tier-aware bullets in their original position.
4. `[LOW]` `internal/exec/quota1_command.go` (`classifyMakeArgs`): allow decimal numbers for `-l`/`--load-average` (e.g. `make -l 1.5 test-short`).
5. `[LOW]` Add test cases in `internal/exec/exec_test.go` and `internal/quota1/quota_test.go` for `-buildvcs ./...`, `-test.short ./...`, `-test.run Foo ./...`, `make -l 1.5 test-short`, and unfinished `TierPackage -> TierFull` remaining blocked.

## 6. Delivery & Review (M2)

- **M2 delivered (`80d0efda`)**: Completed all 5 M1 review pre-work items and updated `EnsureContextUnderThreshold` (`contextTokens <= 0`) + `cmd/harnez/agent_run.go` warning + unit/regression tests in `compact_test.go` and `agent_resume_test.go`.
- **Review (`codex:sol:med`, 1m21s)**: **Ready to accept** (`0 [HIGH]`, `0 [MED]`, `1 [LOW]` optional `-test.bench`/`-test.fuzz` parser test cases).
