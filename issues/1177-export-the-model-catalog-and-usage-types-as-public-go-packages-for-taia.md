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

- **M1 Catalog:** move `internal/modelcatalog` to `ubunatic.com/harnez/modelcatalog` with a `Load`
  entry point; internal callers switch imports. Test: a program outside `internal/` loads the catalog.
- **M2 Quota types:** a `ubunatic.com/harnez/quota` package with only the JSON types of `usage --json`
  (`UsageSummary`, agent and window types) and no imports beyond the standard library; `internal/usage`
  uses them. Pairs with 1176's contract test.
- **M3 Collectors (decide after M2):** split provider quota readers from server, telemetry and TUI so a
  collector can be exported without those deps.

## Owner decisions

- Decided by the owner, 2026-11-06: no `pkg/` directory; packages get real names at the module root
  (`ubunatic.com/harnez/modelcatalog`, `ubunatic.com/harnez/quota`, final names chosen in M1/M2).
  They carry a stable API for taia and other consumers: exported names change only with a
  deprecation step, and the package doc says so.

## Decision

- 2026-11-06 lean sprint (owner: haiku:med developer, terra:low one-shot reviewer/consult). Order:
  1177 M1 (catalog) and M2 (quota types); M3 decided after M2. Then 1173, 1138, 1139 on the same
  branch `sprint/1177-1173-1138-1139`.
- Searcher skipped for 1177: the import-graph findings above were checked live this session and
  name the write set; the developer's plan-only first turn confirms current locations.

### M1 plan accepted (dev-1177, claude:haiku:med)

`Load` exists (`internal/modelcatalog/catalog.go:285`); callers are only in `internal/subagent`; doc
source `docs/other/Spec.md:102` and root copy `docs/Spec.md:102` name the path. Plan: `git mv` the
package with `testdata/`, add `doc.go` (stable-API promise) and an external `modelcatalog_test` that
calls `Load` with temp XDG dirs.

### Pre-Work / Required Refinements (M1)

- Keep every exported name (unexporting after publication would break consumers).
- Mark `Model.Guide` / `LegacyGuide` with a `// Deprecated:` comment naming the replacement field,
  so the stable promise starts with the bridge already on its deprecation path.
- No JSON tags on `Model`/`Policy` now (adding them later is not breaking).
- Edit the doc source `docs/other/Spec.md`, then sync the root copy as `CLAUDE.md` describes
  (`make install`, `harnez init -d .`, `git checkout --` unrelated rewrites).

### M1 delivered (0d8c9607 move, 88241c72 doc and external test)

Host review: 42 files renamed without content change; only import paths in 5 `internal/subagent`
files and the Spec doc path (source and copy) changed. `go test ./modelcatalog/... ./internal/subagent`
green; `make install && harnez init -d .` leaves a clean tree. One-shot reviewer (codex:terra:low,
review-1177-m1): behaviour-neutral; `go list -deps` shows only the root package; no blockers.

### Pre-Work / Required Refinements (M2)

- Strengthen `modelcatalog/load_external_test.go`: assert a known spec model by identity and fields
  (for example `claude-haiku-5-5`: provider `claude`, tier `low`), so a loader with one fabricated
  default fails.
- Resolve the promise mismatch: `doc.go:4` says a `Deprecated:` comment names its replacement, but
  `Model.Guide` (`catalog.go:43`, `:63`) has none. Reword the doc to "names its replacement, or says
  that none exists yet", and keep the Guide marker honest (no pointer to a ticket that does not track
  a replacement).

### M2 Quota types

Goal: a module-root package `ubunatic.com/harnez/quota` with only the JSON types of `usage --json`
(`UsageSummary`, agent and window types), importing nothing beyond the standard library, with the
same package-doc promise. `internal/usage` uses these types (type aliases are fine) so the JSON is
unchanged; the 1176 contract test (`cmd/harnez/usage_json_contract_test.go`) must still pass
unchanged. Add an external test that decodes a `usage --json` sample into `quota.UsageSummary`.
