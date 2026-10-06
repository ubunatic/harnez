# 720 — Split telemetry analytics and datavis out of harnez into a new telemetry project

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor
**Related**: [#204](204-sanitized-telemetry-and-token-export-for-datavis.md) (sanitized export), ubunatic.com issue 071 (`/telemetry/` dashboard retired to `archive/v1/telemetry/`), ubunatic.com `scripts/telemetry/` (`make update-telemetry`)

---

## 1. Problem & Motivation

harnez both collects telemetry (hooks, turn and tool records, ratings, usage) and analyzes it
(stats reports, economics, quality scores). A dashboard (datavis) lives in a third place: the
ubunatic.com `/telemetry/` page, built by ubunatic.com `scripts/telemetry/` from
`harnez usage export --classify`. Owner decision (2026-10-06): keep data collection in harnez and
move analytics and datavis into a new `~/projects/telemetry` project.

## 2. Technical Specification / Findings

- **Stays in harnez (collection):** hooks (`cmd/harnez/hook.go`, `codexhooks.go`), `harnez rate`,
  `internal/telemetry` schema/insert/update/sanitize, `internal/usage` stores, and the sanitized
  export (`harnez usage export`, issue 204) as the stable interface.
- **Candidates to move (analytics):** `harnez stats` and friends (`cmd/harnez/stats*.go`),
  `internal/telemetry` economics, quality, score, query and token-snapshot queries.
- **Moves (datavis):** ubunatic.com `scripts/telemetry/` and the dashboard page (now only in
  ubunatic.com `archive/v1/telemetry/`); the new project publishes it via its `website/` and
  `uman website sync telemetry`.
- **Uncertain:** whether harnez itself still needs some analytics at runtime (e.g. `classify`,
  `score` or economics feeding model/agent decisions, `harnez stats` used in rules and skills).
  Map the call sites before choosing the cut; keep what harnez needs at runtime.
- `internal/usage` imports `internal/telemetry`; the split must not break that.

## 3. Implementation & Verification Plan

/goal harnez keeps collecting and exporting telemetry, while analytics reports and the dashboard
live in `~/projects/telemetry`, which reads only the sanitized export; or stop and report when
blocked on an owner decision (e.g. what harnez must keep at runtime) or a denied permission.

Check the live code first; this ticket may be stale. Plan the cut in this ticket before moving
code, and get the owner's OK on it. Done means: harnez `make check` passes and its remaining stats
needs are met; the new project builds the reports and the dashboard from the export; the
dashboard renders in light and dark theme; ubunatic.com no longer carries `scripts/telemetry/`;
the docs and skills that call moved commands are updated.
