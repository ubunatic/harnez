# 1177 — Export the model catalog and usage types as public Go packages for taia

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 942, 1176, taia#113, taia-plugins `models/`; Jules ideas run `reports/jules/taia-go-library.md`; study docs/studies/2026-11-06-jules-backlog-cleanup.md

---

/goal taia can import harnez's model catalog and usage types as Go packages instead of shelling out and copying structs; or stop and report when blocked on an owner decision (module path, API stability promise).

## Findings (2026-11-06, checked on 9c362dd3)

Jules ideas run `taia-go-library` (GitHub PR 131, 5/5 references held) proposed five seams. Import
graph check by the host:

- `internal/modelcatalog` imports only the root package (spec embed); `go list -deps` counts 2
  harnez packages. Exporting it is a move plus import updates.
- `internal/usage` imports 18 harnez packages, including `internal/server`, `internal/telemetry`,
  `internal/sshutil`, `ubunatic.com/loom` and `ubunatic.com/voxi/audiolevel`. Exporting
  `CollectAll` (`internal/usage/usage.go:84`) or `CaptureTurnQuota` (`internal/usage/turnquota.go:43`)
  as they are would pull all of that into taia.
- taia-plugins `models/harnez.go:17-40` today copies the JSON shape of `UsageSummary`
  (`internal/usage/types.go:243`) by hand.

## Milestones

- **M1 Catalog:** move `internal/modelcatalog` to `pkg/modelcatalog` with a `Load` entry point;
  internal callers switch imports. Test: a program outside `internal/` loads the catalog.
- **M2 Usage types:** a `pkg/usage` package with only the JSON types of `usage --json`
  (`UsageSummary`, agent and window types) and no imports beyond the standard library; `internal/usage`
  uses them. Pairs with 1176's contract test.
- **M3 Collectors (decide after M2):** split provider quota readers from server, telemetry and TUI so a
  collector can be exported without those deps.

## Owner decisions

- Module path and whether `pkg/` carries a stability promise for taia.
