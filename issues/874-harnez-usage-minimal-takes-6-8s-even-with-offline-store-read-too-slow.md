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

## Findings (2026-10-23, after 873, load ~2%)

- `harnez usage --minimal`: 8.2s and 6.1s wall with 0.1s user+sys; `--minimal --offline`: 8.2s wall,
  1% CPU. The command waits, it does not compute; the store-read theory above is likely wrong.
- Suspect: the Remote Load panel (`@x600 · batch`), which printed "remote load unavailable" in the same
  `--offline` run, i.e. an SSH call running into a timeout. Developer test data (no remote host) took 2.0s.
- Required: `--offline` makes no network or SSH call; online, remote load is bounded and never delays the
  local output (fills in later in `-w`, shows "remote load unavailable" at once in one-shot modes).

## Plan

Profile one `--offline` run (pprof or timing per step), fix the dominant step (store queries, file scans,
subprocesses), add a test or benchmark that fails when the read gets slow again.

## Findings (2026-10-23, remote load isolated)

- One-shot modes now read the last remote-load cache entry and never collect remotely; offline watch
  disables the remote-load manager while retaining the panel. Interactive watch continues to start
  the manager in the background.
- A separate local-load panel delay remains: collecting the initial CPU/GPU sparkline history seeds
  samples at 50ms intervals. This was observed in offline output after removing the remote SSH wait;
  it is parked here and was not changed in this milestone. After install, real-store timings were
  2.064s, 2.067s, 2.064s for `--minimal` and 2.062s, 2.062s, 2.063s for
  `--minimal --offline`.
- The hardcoded SSH `ConnectTimeout` and command timeout now come from `spec/usage.yaml` with schema
  validation.
