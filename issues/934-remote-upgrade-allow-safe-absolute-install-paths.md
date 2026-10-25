# 934 — remote upgrade: allow safe absolute install paths

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 928, 919, Epic 921

---

/goal `remote_upgrade.install_path` accepts `~/...` and safe absolute paths (e.g. `/opt/harnez/bin/harnez`,
`/usr/local/bin/harnez`, `/workspace/bin/harnez`) with one rule shared by runtime and schema; `/tmp` and `/var/tmp`
are rejected; an upgrade into an unwritable or noexec target logs a clear reason. Verified by tests and a live
restart with the default path; or stop and report. Circuit breaker: on a loop or no progress, clear the goal and
write a failure report here.

## Background (owner, 2026-10-25)

928 (70072b79) limited the path to `~/` plus `[A-Za-z0-9_./-]`, no `..`. Owner: also allow typical remote and
container-host locations such as `/opt/<tool>/bin`, `/data`, `/work`. Web search w928 (luna:low): `/usr/local/bin`
(Teleport, k3s) and `/opt/<tool>/bin` (FHS) usually need root and are read-only on Fedora CoreOS/Flatcar;
`/work(space)`, `/data`, `/var/...` depend on mount ownership; `/tmp` is often noexec and ephemeral.

## Decision (owner go, 2026-10-25)

- Default stays `~/go/bin/harnez`.
- Allowed: `~/` or `/`, then components of `[A-Za-z0-9_.-]`; no empty or `..` components; no whitespace, controls,
  shell characters or globs. Reject the `/tmp` and `/var/tmp` trees.
- Shell quoting: absolute paths are single-quoted as-is; `~/` keeps the `"$HOME"/'...'` form.
- Failure messages: when mkdir/mv/chmod or the version probe fails with permission denied, read-only filesystem or
  noexec, the log names the path and that reason.
- No sudo; a root-owned target simply fails with that message.

M1 delivered: 973e342a (validator, schema, quoting, failure reasons, server doc).

### M2 Pre-Work (host review of M1)

- `.` components pass: `/./tmp/x` and `/var/./tmp/x` bypass the temp-tree rejection. Reject `.` components (runtime
  and schema), add these two plus `~/./x` and `/a/./b` as invalid cases.

## Write set

`internal/sshutil/options.go` (+ test), `internal/server/remote_upgrade.go` (+ test), `spec/schemas/server.schema.json`,
docs that describe `install_path` (find with `harnez find`).
