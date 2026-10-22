# 862 — harnez server restart, logs, status --json, uptime and version in status

**Status**: Open
**Priority**: P2 (Medium) — after `make install` the server keeps running the old binary and nothing shows it
**Severity**: Minor
**Category**: Feature
**Related**: 861 (server stop), 837 (server over Unix socket), 825 (serve epic)

---

/goal `harnez server` gains `restart`, `logs`, `status --json`, and status shows uptime and the running
build's version, each delivered as its own milestone commit with tests, docs and help golden. Verified
with a live run of every subcommand on the installed binary, or stop and report when blocked.

## Facts (2026-10-22)

- Subcommands in `cmd/harnez/server_cmd.go`: `install`, `uninstall`, `status`, `stop` (861, `stopLocalServer`).
- `internal/server/systemd.go`: `SystemdService` with `Install`, `Uninstall`, `HandOver`, `Status`, `Stop`.
- Socket routes (`internal/server/server.go:85-87`): `GET /status` (version, identity, ready), `POST /shutdown`.
- Timeouts and unit name come from `spec/server.yaml`; no new Go defaults (docs/Spec.md).
- Docs naming server subcommands: `docs/HarnezServerArchitecture.md`, `docs/UsageCollection.md`; help golden
  `cmd/harnez/testdata/help.golden`.

## Milestones

- **M1 (restart):** `harnez server restart`. systemd-owned → `systemctl --user restart <unit>`; foreground →
  stop as in 861, then start the server the way `harnez server` starts it detached (if no detached start exists,
  report and restart only the systemd path); not running → start it (unit if installed). Tests with fake runner.
- **M2 (uptime and version in status):** `/status` reports start time; `harnez server status` prints uptime and
  the server's version, and flags when it differs from the CLI's version ("restart to update"). Tests for
  equal and different versions, server not running.
- **M3 (status --json):** one JSON object with unit (installed, active, main_pid), socket owner, pid, version,
  uptime_seconds, version_mismatch. Not running → valid JSON with running=false, exit 0. Golden-style test.
- **M4 (logs):** `harnez server logs [-f] [-n N]` → `journalctl --user -u <unit>`; without systemd say where
  foreground logs go and exit 0. Default line count from spec if a value is needed. Test with fake runner.
