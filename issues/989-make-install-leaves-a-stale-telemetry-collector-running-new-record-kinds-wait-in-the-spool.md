# 989 — make install leaves a stale telemetry collector running; new record kinds wait in the spool

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 987; goself 006 (generic restart detection after install)

---

/goal A newly installed harnez never leaves a telemetry record kind unwritten because the running collector is
older than the client: either the collector is restarted (or warns) after install, or the client reports the
version mismatch where the user sees it. Stop and report if the fix needs a change to how the systemd user
service is managed that the owner has not approved.

## 1. Problem & Motivation

In the 987 M4b live check, `harnez agent compact` on an agy session sent a new record kind (`agy_compaction`) to
the collector (`harnez-agent-collector.service`). The collector was still the build from before `make install`
(`0.2.3+3853`; installed binary `0.2.3+3978`), so the record stayed in `~/.local/state/harnez/telemetry-spool`
and `agent status --json` showed `"compaction_pair": null`. Nothing said why. After `harnez server restart` the
pair appeared. Any feature that adds a record kind hits this, and the developer's tests pass because they do
not go through the running service.

## 2. Findings

- The compact command succeeded and printed its result; only the stored record was missing.
- The spool kept the record, so no data was lost; it was written after the restart.
- `harnez usage` already takes over from an older server (docs/UsageCollection.md), and `harnez server status`
  flags a version mismatch; telemetry senders do neither.

## 3. Options

1. `make install` runs `harnez server restart` when the collector service is active.
2. The client compares its version with the collector's on send and prints one stderr line on mismatch.
3. The collector rejects unknown kinds with an explicit error that the client surfaces.

Verify: install a build with a new kind while an older collector runs; the record lands, or the user is told why.
