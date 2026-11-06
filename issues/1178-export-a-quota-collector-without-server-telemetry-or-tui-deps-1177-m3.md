# 1178 — Export a quota collector without server, telemetry or TUI deps (1177 M3)

**Status**: Backlog
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: 1177, 942, taia#113; study docs/studies/2026-11-06-two-agent-lean-sprint-public-packages.md

---

/goal taia can read provider quota windows in-process through a harnez package that imports only `quota`, `modelcatalog` and the standard library; or stop and report when blocked on an owner decision.

## Context

1177 M1 and M2 shipped `ubunatic.com/harnez/modelcatalog` and `ubunatic.com/harnez/quota` (types only).
Collection stays in `internal/usage`: `CollectAll` (`internal/usage/usage.go:84`) and `CaptureTurnQuota`
(`internal/usage/turnquota.go:43`) pull in 18 harnez packages, including server, telemetry, ssh, loom and
voxi (checked 2026-11-06). taia-models shells out to `harnez usage --json --offline` today, which works.

## Want

- Split provider quota readers from server, telemetry and TUI code, then export a collector package
  with the same stable-API promise.

## Done when

- `go list -deps` of the exported collector shows no `internal/server`, `internal/telemetry`, loom or voxi,
  and a test reads a fixture provider's windows through it.
