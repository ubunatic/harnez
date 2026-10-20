# 828 — harnez usage reports 'database is locked' instead of waiting for or recovering from the store lock

**Status**: Closed — Confirmed compact usage performed repeat DDL, the key migration transaction, compat checks, and observation/history writes; current-schema checks now skip completed migrations and short-timeout busy writes are logged while projection continues. Added held-lock and no-lock regressions plus a compact busy fallback and spec key. make install passed. The Quota-1 suite passed internal/usage but failed the key-migration repair test; the repair fast path was corrected afterward but not rerun under the single-run quota.
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

### Searcher findings (sprint 2026-10-20)
- `usage` compact and `--watch` project via `storeCompactSummary` (`internal/usage/store.go:24-31,487-500`),
  opening with `usagestore.Open`: WAL plus a **5s** busy timeout (`internal/usagestore/store.go:79-100`).
  Statusline writers use a 25ms timeout (`internal/usage/passive.go:52-90`).
- Writer transactions (`WriteCurrent`, `internal/usagestore/store.go:402-439`) do no network/file I/O.
- There is no retry or last-known fallback (`internal/usage/store.go:613-633`). debug.log shows 13 failures,
  the last ones on Oct 6, 8 and 9, all `SQLITE_BUSY`.
- Open question: a WAL reader should not hit SQLITE_BUSY after 5s. Does the read path write
  (schema migration in `Open`, mirror/compat writes, checkpoints)? Is the busy timeout actually applied to every
  pooled connection? Find the real cause first.

## 3. Implementation & Verification Plan
- Give the interactive read path a spec-configured busy timeout and/or a bounded retry; on
  failure show last-known data with one compact "store busy" note instead of per-row raw errors.
- Shorten writer transactions if one turns out to hold the lock long.
- Test with a blocker connection holding a write lock (pattern exists in `passive_test.go`).
