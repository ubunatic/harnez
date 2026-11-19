# 1250 — telemetry record dropped prints on every harnez call inside a taia sandbox

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1181, 989

---

/goal Under taia, harnez CLI invocation telemetry either reaches the server or the spool, or is dropped quietly, with at most one warning per session; find out why the socket POST fails and add the spool path to the taia harnez profile if that is the fix, or stop and report when dropping telemetry needs a policy decision. On a loop, hard blocker, or no progress, clear the goal and write a failure report.

## 1. Problem & Motivation

Reported from a cati session (inbox draft `harnez-telemetry-dropped-noise-under-taia.md`, 2026-11-19) and reproduced in this harnez session (Claude Code under taia, 2026-11-19). Every `harnez` call (`read`, `find`, `issues new`, ...) ends with:

```
harnez: telemetry record dropped (broker channel or host spool unavailable/full)
```

This adds noise to every tool result and costs context. Agents start filtering it with `grep -v telemetry`. The telemetry is also really lost.

## 2. Technical Specification / Findings

Path: `cmd/harnez/telemetry_server.go:310-328` `sendTelemetryRecord` -> `server.DispatchTelemetry` (`internal/server/runner_channel.go:189`). This session is not in runner mode, so it uses host mode: `PostTelemetry` (`internal/server/client.go:39`) POSTs to the host socket and, if that fails, appends to the host spool. The record counts as dropped only when **both** fail.

Probed in this sandbox (2026-11-19):

- Spool: `~/.local/state/harnez/telemetry-spool` (`Config.TelemetrySpoolDir`, `internal/server/spec.go:220`) exists, but taia's write list does not include `~/.local/state/harnez`, so `touch` gives `Permission denied`. The list has `~/.harnez/{agents,agymeter,feedback,run,sessions}`, `~/.local/share/harnez`, `~/.cache/harnez`, `/tmp/harnez` and `/run/user/1000/harnez`.
- Socket: `/run/user/1000/harnez/server.sock` is reachable. `curl --unix-socket ... http://x/` returns 404, and the server (`harnez server`, started 15:33) runs the same build as `~/go/bin/harnez`. **So the POST fails for a reason not found yet:** maybe non-2xx from the telemetry path (peer or session validation of sandboxed callers?), a timeout, or a payload rejection. `PostUpdate` drops the status, so the CLI cannot tell which.
- The message is printed on every invocation; there is no once-per-session limit.

## 3. Implementation & Verification Plan

- [ ] Root cause the POST failure from inside taia. Log the HTTP status or dial error at debug level in `PostUpdate` / `PostTelemetry`, and check the server log for the rejected request.
- [ ] Fix whichever applies: make the server accept sandboxed CLI telemetry, and/or add `~/.local/state/harnez/telemetry-spool` (only that directory) to the taia harnez profile or the harnez-generated sandbox grants.
- [ ] Rate-limit the stderr line: at most once per session (marker under the session state dir), otherwise debug-only. Policy decision for the owner: should a dropped CLI-invocation record be silent by default?
- [ ] Test: dispatch with an unwritable spool and a failing socket prints the warning once across two calls in the same session.
