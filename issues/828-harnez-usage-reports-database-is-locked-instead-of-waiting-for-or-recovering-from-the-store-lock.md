# 828 — harnez usage reports 'database is locked' instead of waiting for or recovering from the store lock

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 660, 792

---

/goal `harnez usage` never shows a raw `database is locked` error for a transient writer lock: it waits briefly, retries, or falls back to the last known data with a short stale hint, and a regression test covers a held lock. Stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
`harnez usage` sometimes renders every provider row as unavailable with the raw SQLite error:

```
Claude/GPT unavailable (store projection failed: database is locked (5) (...
Gemini 4 Argon unavailable (store projection failed: database is locked (...
Gemini unavailable (store projection failed: database is locked (5) (SQLI...
Other AGY Models unavailable (store projection failed: database is locked...
```

This is bad UX: one short-lived writer (collector, statusline observation) blanks the whole
table, the truncated error text is noise to the user, and the lock seems to take too long to clear.

## 2. Technical Specification / Findings
- The message comes from `compactProjectionFailure` in `internal/usage/store.go`, which copies the
  projection error into `QuotaFetchError` for every provider.
- `spec` has `statusline_busy_timeout` (1ns–1s); check whether the `usage` read path uses any busy
  timeout at all, or a too-short one, and whether a writer holds its transaction too long.
- Uncertain: which writer holds the lock in the observed cases (collector vs. statusline vs. a
  concurrent `usage --watch`). Check `~/.harnez/debug.log` (`usage compact projection failed`).

## 3. Implementation & Verification Plan
- Give the interactive read path a spec-configured busy timeout and/or a bounded retry; on
  failure show last-known data with one compact "store busy" note instead of per-row raw errors.
- Shorten writer transactions if one turns out to hold the lock long.
- Test with a blocker connection holding a write lock (pattern exists in `passive_test.go`).
