# 418 — Deferred refinements and architectural polish from 416 post-tool telemetry milestones

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Low
**Category**: Telemetry / Refactor & Architecture
**Related**: #416, #417, `docs/practices/AgenticLoop.md`

---

## 1. Problem Statement & Motivation

During the execution of Issue #416 milestones, the Host Orchestrator applies the "Nuance Collector & Deferred Refinement Gate" pattern. Non-blocking nuances, minor performance optimizations, and architectural enhancements identified during inline diff reviews are collected here rather than incurring costly round-trip context disruptions during subagent milestone delivery.

If the buffer of refinements reaches an architectural tipping point during the sprint, the orchestrator intercepts to sweep them; otherwise, this ticket serves as the consolidated post-milestone refinement backlog.

---

## 2. Collected Refinements

### From Milestone 1 (Database Schema & Telemetry Migration)
1. **Migration Version Guard**:
   - In `internal/telemetry/telemetry.go`, `migrateToolCalls` currently executes a PRAGMA table info check on every `Open()`.
   - *Refinement*: Wrap migration logic behind `if current < schemaVersion` so warm database opens bypass PRAGMA checks once already on the current version.
2. **Step-Versioned Migrations**:
   - *Refinement*: Structure database schema migrations into discrete version transitions (e.g. `migrateV2ToV3(sqlDB)`) rather than checking an unstructured list of all historical columns on every version bump.

### From Milestone 2 (Post-Tool Hook Handler & Correlation Engine)
3. **Hook Fail-Open Resiliency**:
   - In `runAgyPostToolHook`, a decode or DB error returns a non-zero exit without writing `{"status":"ok"}`.
   - *Refinement*: Always emit `{"status":"ok"}` to stdout even on non-critical decode/DB errors (logging to stderr instead) so agent harnesses never stall if telemetry fails.
4. **Transcript Step Parsing vs Whole-File Ingestion**:
   - When `payload.Output` is empty and `payload.TranscriptPath` is provided, `os.ReadFile(payload.TranscriptPath)` calculates bytes over the *entire conversation file* rather than the specific tool output step.
   - *Refinement*: Tail/parse the last step JSON from the transcript file (or use step index) rather than measuring the cumulative transcript file length.
5. **Delta Duration Calculation**:
   - Duration is currently passed as `0`.
   - *Refinement*: Compute $\Delta t = \text{now} - t_{\text{created\_at}}$ (either in Go or SQLite `strftime`) to store true execution duration in `duration_ms`.

---

## 3. Verification Target
- `go test -v ./internal/telemetry/...` verifies both fresh database initialization and schema migration from v1/v2 to v3 with version-guarded checks.
