# 931 — remote: ssh-agent socket under PrivateTmp and remote-load retry durations in spec

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 920, 929, 928, Epic 921 (review r920, 2026-10-25)

---

/goal The server unit reaches an ssh agent whose socket lives under `/tmp`, or documents and logs why not, and the
remote-load retry durations come from `spec/` with a schema; or stop and report. Circuit breaker: on a loop or no
progress, clear the goal and write a failure report here.

## Findings (reviewer r920, sol:low)

- `internal/server/systemd.go:95`, `systemd/harnez-agent-collector.service:18`: an `SSH_AUTH_SOCK` captured at apply
  time under `/tmp` is not reachable inside the unit (`PrivateTmp=yes`). Today x600 works by key without an agent,
  so this only bites agent-only setups.
- `internal/usage/collector_remote_load.go:13`: retry/backoff durations are Go constants; move them to
  `spec/usage.yaml` with schema (per `docs/Spec.md`).
