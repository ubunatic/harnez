# 652 — Register Active Usage Collectors

**Status**: Closed — Registered provider, AGY meter, and remote-load collectors with normalized store publishing and verified usage output.
**Priority**: P1 (High)
**Severity**: Major
**Category**: Architecture
**Related**: [650](650-usage-collection-architecture-umbrella.md), [UsageCollection.md](../docs/UsageCollection.md)

---

/goal Move active provider, AGY-meter, and remote-load gathering behind the usage collector registry, or stop and report when blocked on a user decision or denied permission.

## Acceptance

- Registered collectors declare identity, capabilities, cadence, timeout and cancellation.
- Claude, Codex, AGY, AGY meter and remote-load publish normalized observations.
- Slow collection does not delay independent collectors; diagnostics retain source timing/error evidence.
- Retire direct `CollectAll` reader orchestration only after equivalent snapshot and fallback behavior is verified.

## Implementation Outcome

- Registered Claude, Codex, AGY, the passive AGY meter source, and remote load with cadence and timeout policy. Remote load samples use the additive `usage_load_observations` table; quota collectors publish normalized windows through the usage store.
- The `compat-v1` marker already gates the compatibility importer, so the backfill remains one-time.
- Replaced `collectAllWithDiagnostics`' direct fan-out/cache-first branches with registry collection while preserving compatibility wrappers and snapshot/history fallbacks.
- Commits: `e9d7226b`, `e9b4a346`, `9a6ecd7c`.
- Verification: `go test ./internal/usage/... ./internal/usagestore/` and `go test ./cmd/harnez/ -run Usage` passed; `make install` passed.
