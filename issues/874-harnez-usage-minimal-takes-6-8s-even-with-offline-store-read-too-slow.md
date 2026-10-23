# 874 — harnez usage --minimal takes 6-8s even with --offline: store read too slow

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 873 (never hang, 10s limit), 660 (earlier slow start), 113 (collector roundtrip times)

---

/goal `harnez usage --minimal` and `--offline` print in under 1 second on the user's host with the real store,
with the slow step found and fixed and a timing check that guards it; or stop and report what dominates if it
cannot be fixed inside harnez.

## Findings (2026-10-23, v0.2.0 + b47b1263)

- `harnez usage --minimal`: 6.1s, 6.1s, 8.2s; `harnez usage --minimal --offline`: 8.2s; `harnez server status`:
  0.08s. So the time goes into reading and building the summary locally (`loadLocalUsage` ->
  `usage.ReadStoredSummary`, `cmd/harnez/usage_client.go`), not the server.
- Load average was 4-5 (another session running heavy tests); measure again on an idle host before blaming code.

## Plan

Profile one `--offline` run (pprof or timing per step), fix the dominant step (store queries, file scans,
subprocesses), add a test or benchmark that fails when the read gets slow again.
