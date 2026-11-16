# 1234 — harnez usage silently reads from an older harnez server without warning or update

**Status**: Closed — usagecli detects older harnez server via CompareBuild, triggers non-blocking handover/restart and prints status; make install restarts active harnez-agent-collector.service; make test-q1 green, make install and live harnez usage verified Claude fable weekly at 27%
**Closed-By**: session d3745c98-8c17-42c0-ab28-f6f0104b79c2
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 989, 1230, 837, 862, docs/UsageCollection.md, docs/HarnezServerArchitecture.md

---

## 1. Problem & Motivation
Running `harnez usage` (`0.5.8+5234`) omitted Claude's Fable weekly quota (`extra_windows.fable`, implemented in #1230) even though `api.anthropic.com/api/oauth/usage` returned `weekly_scoped` for `"Fable"` (`27%`).

Investigation showed `harnez-agent-collector.service` (`harnez server`, PID `206488`) had been running continuously since `2026-11-12` on version `0.5.7+5148` (before #1230). Two gaps caused the silent failure:
1. `make install` updated `~/go/bin/harnez` without restarting an active `harnez-agent-collector.service` (also noted in #989 for telemetry).
2. `internal/usagecli/client.go` (`probeHarnezServer` / `ServerStarter.Ensure`) checked only `status.Health == "ok"` and ignored `status.Version` and `status.BuildIdentity`, so `harnez usage` neither warned the user nor triggered `harnez server` handover/takeover when the running server was older.

## 2. Technical Specification / Findings
- `internal/server/server.go` already implements `compareBuild(versionA, identityA, versionB, identityB)` and `claimExisting`, which hands over to systemd (`HandOver`) or shuts down a stray older server when `harnez server` runs from a newer build.
- `internal/usagecli/client.go` decodes `status.Version` and `status.BuildIdentity` from `GET /status` in `probeHarnezServer` but discards them.
- Exporting `server.CompareBuild` lets `probeHarnezServer` detect when the CLI binary is newer (`CompareBuild > 0`) and report `outdated` + `serverVersion`.
- `ServerStarter.Ensure` triggers `s.start()` (non-blocking detached `harnez server` for handover/takeover) and reports `server updating (<old> -> <new>)` on stderr (or in `harnez usage -w`'s status footer), or `server outdated (...)` if restarting fails.
- `Makefile` `install` restarts `harnez-agent-collector.service` when systemd reports it active.

## 3. Implementation & Verification Plan
- Export `server.CompareBuild` in `internal/server/server.go`.
- Update `internal/usagecli/client.go` to detect outdated servers via `server.CompareBuild`, trigger `s.start()`, and surface the status message; add unit tests in `internal/usagecli/client_test.go` (including an `httptest` server probe test).
- Update `Makefile` `install` to restart active `harnez-agent-collector.service`.
- Verify with `make test-q1`, `make install`, `harnez server status`, and live `harnez usage` / `harnez usage --json` confirming `fable weekly` / `extra_windows.fable` appears.
