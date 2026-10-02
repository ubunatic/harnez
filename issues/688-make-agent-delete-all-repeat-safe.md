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
