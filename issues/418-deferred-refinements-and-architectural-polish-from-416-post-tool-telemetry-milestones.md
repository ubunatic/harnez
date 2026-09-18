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
3. **Hook Fail-Open Resiliency & Protojson Contract**:
   - In `runAgyPostToolHook`, a decode or DB error should never fail the process exit code or emit unmarshalable JSON.
   - *Refinement*: Always emit `{}` on stdout even on non-critical decode/DB errors (logging to stderr instead) so Antigravity's protojson unmarshaler succeeds and agent harnesses never stall if telemetry write fails.
4. **Transcript Step Parsing vs Whole-File Ingestion**:
   - When `payload.Output` is empty and `payload.TranscriptPath` is provided, `os.ReadFile(payload.TranscriptPath)` calculates bytes over the *entire conversation file* rather than the specific tool output step.
   - *Refinement*: Tail/parse the last step JSON from the transcript file (or use step index) rather than measuring the cumulative transcript file length.
5. **Delta Duration Calculation**:
   - Duration is currently passed as `0`.
   - *Refinement*: Compute $\Delta t = \text{now} - t_{\text{created\_at}}$ (either in Go or SQLite `strftime`) to store true execution duration in `duration_ms`.

### From Milestone 3 (Potential Opportunity Savings Evaluator for Native Reads)
6. **Provider-Aware Opportunity Cost Evaluation**:
   - In `runAgyPostToolHook`, `readcard.EstimateSavings` hardcodes `readcard.ProviderClaude`.
   - *Refinement*: Infer the provider model profile from `calls[0].AgentID` (e.g. `agy` maps to Gemini ViT pricing, `claude` maps to Claude ViT pricing, `codex` maps to OpenAI ViT pricing).
7. **Savings Calculation from Transcript Fallback**:
   - If `payload.Output` is empty but `payload.TranscriptPath` is supplied, `EstimateSavings` is bypassed.
   - *Refinement*: Extract the tool output text from the transcript step before passing to `EstimateSavings`.

### Universal Tool Output Token Estimation
8. **Always Compute Estimated Tokens Across All Tool Invocations**:
   - In `runAgyPostToolHook`, calculate estimated output tokens for *every* tool invocation (`actual_tokens = ComputeTextTokens(payload.Output).TextTokens`) and record it in `tool_calls.actual_tokens`.
   - *Value*: Allows benchmarking heuristic token estimates directly against ground-truth provider turn deltas for every command/tool execution in real-world workflows.

### From Milestone 5 (Analytical Reporting in `harnez stats`)
9. **Zero vs N/A Table Formatting for Savings**:
   - In `cmd/harnez/stats.go`, zero potential savings currently outputs `0`.
   - *Refinement*: Render `-` or `n/a` when no savings data exists for cleaner terminal visual scanning.
10. **JSON Field Tagging Consistency**:
    - *Refinement*: Ensure `GroupStats` JSON export tags strictly match snake_case conventions (`avg_actual_tokens`, `potential_savings_tokens`).

---

## 3. Verification Target
- `go test -v ./internal/telemetry/...` verifies both fresh database initialization and schema migration from v1/v2 to v3 with version-guarded checks.
