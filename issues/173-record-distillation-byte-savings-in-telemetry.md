# 173 — Record Distillation Byte Savings in Tool Telemetry

**Status**: Closed — resolved in distillation metrics telemetry pipeline
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Telemetry / Distillation
**Related**: [[116-tool-telemetry-schema-and-storage-layer]], [[118-harnez-exec-shell-interceptor]], [[120-harnez-stats-analytical-reporting]], `cmd/harnez/distill.go`, `cmd/harnez/exec.go`, `internal/telemetry/telemetry.go`

---

## 1. Problem & Motivation

`harnez stats` reports:
```text
distillation byte savings: no rows with distillation data
```

While `tool_calls` includes a `distilled_bytes` column and `harnez stats` computes a global savings ratio `(1 - distilled_bytes/raw_bytes)`, `harnez exec` and `harnez distill` currently leave `distilled_bytes` as `NULL`. Because distillation byte counts are never captured during execution, the telemetry pipeline cannot measure token or bandwidth savings from output filtering.

## 2. Technical Specification

1. **Expose Byte Metrics in `internal/distill`**:
   - Extend distillation execution helpers to return or track `rawBytes` vs `distilledBytes`.
2. **Bridge Distillation into `harnez exec`**:
   - When `harnez exec` wraps a command whose output is piped through `distill` (or when `distill` executes in wrapper mode `harnez distill -- <cmd>`), capture both uncompressed `raw_bytes` and compressed `distilled_bytes`.
3. **Telemetry Insertion**:
   - Write `distilled_bytes` into the `tool_calls` row on SQLite insert.
4. **`harnez stats` Reporting**:
   - Ensure `harnez stats` calculates and displays the aggregate byte savings percentage and total bytes saved across recorded sessions.

## 3. Constraints & Edge Cases
- If a command output is not distilled (or filter mode is `raw`), `distilled_bytes` should remain `NULL` (or equal `raw_bytes`) without skewing ratios.
- Distillation metrics collection must not add buffering delay or affect the child process's streaming stdio behavior.

## 4. Verification Plan

- [x] Run a command wrapped in `harnez distill -- <cmd>` or `harnez exec` with distillation enabled.
- [x] Query `~/.harnez/tool_catalog.sqlite` to verify non-null `distilled_bytes` in `tool_calls`.
- [x] Run `harnez stats` and verify the `distillation byte savings` line reports valid percentage and byte metrics.
