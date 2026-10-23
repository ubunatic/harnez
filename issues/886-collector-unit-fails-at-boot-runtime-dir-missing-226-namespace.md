# 886 — Collector unit fails at boot: runtime dir missing (226/NAMESPACE)

**Status**: Closed — Added RuntimeDirectory=harnez and RuntimeDirectoryPreserve=yes to the embedded unit; regression test failed before fix and go test ./internal/server plus make install passed. Reinstalled via harnez server install; daemon-reload, restart and stop/start succeeded, active with NRestarts=0 and no 226/NAMESPACE in the verification journal. Missing-directory boot simulation skipped because runtime/procs contains live process records used by the active Harnez session; directory preserved.
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 082 (collector daemon)

---

/goal Make `harnez-agent-collector.service` start on the first try after a reboot and verify it,
or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
After a reboot the usage UI (`harnez usage -w`) showed everything gray for about a minute.
The collector server was not running: systemd failed to start it 13 times in a row
(every 5 seconds, ~70 s total), and it only came up once some other process had created
`/run/user/1000/harnez`.

## 2. Technical Specification / Findings
Observed on 2026-10-23 (journal for the current boot):

- 19:49:24 first start; every attempt failed with
  `Failed to set up mount namespacing: /run/user/1000/harnez: No such file or directory`,
  exit `226/NAMESPACE`.
- Cause: `systemd/harnez-agent-collector.service` lists `%t/harnez` in `ReadWritePaths`.
  `%t` is `/run/user/<uid>`, a tmpfs wiped on every boot; systemd refuses to set up the
  sandbox when a listed path does not exist.
- 19:50:32 attempt 14 succeeded, after the directory was created by another harnez process
  (likely `harnez mcp`/`harnez channel` started by Claude Code; not verified).
- 19:50:46 usage UI started, so its own fallback (`startDetachedServer`,
  `cmd/harnez/usage_client.go`) never ran.
- 19:50:49 `catch-up fetch (clients 0->1)` fetched all providers: the fetch-on-first-client
  behaviour works; only the server start was late.

## 3. Implementation & Verification Plan
- Add `RuntimeDirectory=harnez` and `RuntimeDirectoryPreserve=yes` to the unit, so systemd
  creates the directory before the sandbox is set up (and does not delete it on stop,
  since clients also use it).
- Extend `internal/server/systemd_test.go` to assert both directives.
- `make install`, reinstall the unit, reboot (or remove `/run/user/$UID/harnez` and restart
  the service) and confirm the journal shows one clean start.
