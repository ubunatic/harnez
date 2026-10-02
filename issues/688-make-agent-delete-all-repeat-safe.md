# 688 — Make agent delete all repeat-safe

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: [Codex provider deletion](../internal/subagent/codex.go)

---

## 1. Problem & Motivation
After `harnez agent delete --all`, running it again can show the same sessions
again, including Codex sessions whose provider-side delete returned an error.
Repeated cleanup should converge instead of presenting the same agents on
every invocation.

## 2. Technical Specification / Findings
Make `delete --all` repeat-safe: sessions handled by one invocation should not
be listed again by the next invocation. Preserve a clear way to inspect or
retry unresolved provider deletions without repeating them as ordinary pending
sessions on every global delete.

## 3. Implementation & Verification Plan
**Goal**: Ensure repeated `harnez agent delete --all` runs do not show agents
already handled by an earlier run. Done when a second run skips prior entries,
including provider failures, while unresolved failures remain explicitly
recoverable and are not silently mistaken for confirmed provider deletion.
Stop and report if a provider cannot distinguish a deleted session from an
unresolved one.

## 4. Decisions & Pre-Work (2026-10-02)
- **Stop condition hit, resolved by user go:** `ClaudeDriver.Delete` and `AgyDriver.Delete`
  are no-ops returning nil, so no provider-side deletion is confirmed for them. They are
  reported as "removed from harnez only", never as provider-confirmed deletion.
- **User requirement 1:** `harnez agent delete --all --force` prints no "unrated latest turns"
  warning (today `printUnratedDeleteWarning` runs even with `--force`, `cmd/harnez/agent.go`).
  Without `--force` the warning and refusal stay as they are.
- **User requirement 2:** names deleted by one run never reappear in the next run or in the
  default `agent list`, for every provider. A Codex provider-delete failure keeps a record
  marked `delete-failed` (with the error) that is hidden from `delete --all` and the default
  list, shown only via an explicit recovery path (`delete --retry-failed`, and a list flag
  such as `--failed`); a successful retry removes it.
