# 861 — harnez server stop: stop the running local server cleanly

**Status**: Closed — harnez server stop: systemd unit stopped via systemctl (unit stays enabled), foreground server via POST /shutdown plus socket wait with spec timeouts, not running exits 0; host live check stop/status/restart passed (e17c10a2)
**Priority**: P2 (Medium) — the server has `install`, `status`, `uninstall`, but no way to stop it without
removing the service or killing a PID
**Severity**: Minor
**Category**: Feature
**Related**: 837 (server over Unix socket, takeover), 825 (serve epic)

---

/goal `harnez server stop` stops the running local server and reports what it did: when the systemd user
unit owns the server, `systemctl --user stop <unit>` (the unit stays installed and enabled); otherwise
`POST /shutdown` on the socket, then wait until the socket stops answering. No server running → say so,
exit 0. Verified with tests and a live run, or stop and report when blocked.

## Facts (2026-10-22)

- `internal/server/server.go:87` already serves `POST /shutdown`; nothing in the CLI calls it.
- `internal/server/systemd.go` has `Install`, `Uninstall`, `HandOver`, `Status`; no `Stop`.
- `cmd/harnez/server_cmd.go` has `install`, `uninstall`, `status`; `localServerPID` finds the socket owner.
- Timeouts come from `spec/server.yaml` (`shutdown_timeout`, `startup_poll_interval`, `http_client_timeout`);
  no new Go defaults (docs/Spec.md).

## Checks

- Unit tests: systemd-owned → stop via systemctl (fake runner); foreground server → shutdown request and the
  wait ends when the socket is gone; nothing running → "not running", exit 0; wait timeout → clear error.
- Live: `harnez server status` before and after `harnez server stop`.
- Docs: the server command in docs/CLIDesign.md (or wherever server subcommands are documented), man/help
  golden if affected.
