# 416 — Implement post-tool lifecycle hook and telemetry pipeline for actual token delta and potential read savings

**Status**: Closed — All 5 milestones implemented, tested, and verified
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Hooks & Telemetry / Observability
**Related**: #405, #412, #296, #173, #120, #142, `docs/TokenMeasurementArchitecture.md`

---

## 1. Problem Statement & Motivation

Currently, Harnez relies exclusively on `PreToolUse` hooks (such as `harnez hook agy` and `harnez guard`) to intercept and observe agent tool invocations before execution. While this permits gating and basic call logging into `tool_catalog.sqlite`, it leaves significant observability gaps:

1. **No Post-Execution Visibility**: Once a tool executes, Harnez has no `PostToolUse` or `PostInvocation` hook listening to inspect tool output payloads, execution status, or response sizes.
2. **Missing Ground-Truth Token Telemetry**: Real token costs for individual tool calls (including `harnez read -I`, `view_file`, `grep_search`, `run_command`) are neither measured nor stored in telemetry. `harnez stats` only reports text distillation byte savings from `harnez distill`, leaving vision and tool token deltas unrecorded.
3. **No Native Tool Opportunity Cost Tracking**: When agents invoke native file-reading tools (e.g. `view_file`, `ReadMultipleFiles`, `cat`), Harnez does not quantify the *potential* token savings that would have been achieved had the agent used `harnez read -I` or `harnez read -L`.

Architecture reference: [`docs/TokenMeasurementArchitecture.md`](../docs/TokenMeasurementArchitecture.md).

---

## 2. Measurable Development Milestones

### Milestone 1: Database Schema & Telemetry Migration
- **Goal**: Extend the telemetry schema to store execution metrics, output sizes, token costs, and potential savings.
- **Scope**:
  - `internal/telemetry/types.go`: Add `OutputBytes *int64`, `ActualTokens *int64`, `PotentialSavingsTokens *int64`, `PotentialSavingsBytes *int64` to `ToolCall` struct.
  - `internal/telemetry/schema.go`: Update DDL to include the 4 new columns on `tool_calls`. Add automated migration in `Open` / `EnsureSchema` to execute `ALTER TABLE tool_calls ADD COLUMN ...` on existing databases without data loss.
  - `internal/telemetry/insert.go` & `query.go`: Update insert statements, scan queries, and export filters.
- **Verification Target**:
  - `go test -v ./internal/telemetry/...` passes, including migration tests with legacy databases and insert/query roundtrips for all new columns.

---

### Milestone 2: Post-Tool Hook Handler & Correlation Engine
- **Goal**: Implement the `PostToolUse` CLI entrypoint in Harnez that correlates with the preceding `PreToolUse` record and captures execution duration and payload sizes.
- **Scope**:
  - `cmd/harnez/hook.go`: Add `harnez hook post-tool` (and alias `harnez hook agy-post`).
  - Input: Stdin JSON with `conversationId`, `stepIdx`, `transcriptPath`, `error` (standard AGY/Antigravity payload).
  - Correlation logic: Match the `tool_calls` row by `(session_id, stepIdx)` (or recent pending record).
  - Output extraction: Read the tool output step from `transcriptPath` (or stdin payload), calculate `output_bytes` and duration $\Delta t = t_1 - t_0$.
  - Update SQLite row with measured output metrics.
  - Output: `{"status":"ok"}` or `{}` on stdout.
- **Verification Target**:
  - `go test -v ./cmd/harnez/hook_test.go` simulates sequential `PreToolUse` -> tool execution -> `PostToolUse` and confirms `output_bytes` and duration are persisted in the database.

---

### Milestone 3: Potential Opportunity Savings Evaluator for Native Reads
- **Goal**: Automatically calculate what a native read tool (`view_file`, `cat`, `ReadMultipleFiles`) would have cost if rendered with `harnez read -I`.
- **Scope**:
  - `internal/readcard/` / `internal/telemetry/`: Expose helper `EstimateSavings(textPayload string, provider string) (textTokens, vitTokens, savingsTokens, savingsBytes int)`.
  - When `PostToolUse` processes a native file-reading tool, tokenize the returned text payload and compute hypothetical ViT tile tokens for an equivalent visual card.
  - Set `potential_savings_tokens = max(0, textTokens - vitTokens)` and `potential_savings_bytes`.
- **Verification Target**:
  - Unit tests with known text fixtures (>300 lines) verifying positive `potential_savings_tokens` calculated against Claude and OpenAI ViT pricing.

---

### Milestone 4: Hook Manifests Generation & Agent Integration
- **Goal**: Ensure `harnez init` and `harnez apply` register both `PreToolUse` and `PostToolUse` hooks in agent configuration files.
- **Scope**:
  - `internal/agy/hooks.go`: Update `BuildHooksDoc()` to include `PostToolUse` with matcher `*` calling `harnez hook agy-post` alongside `PreToolUse`.
  - `internal/claude/hooks.go`: Register post-tool hooks for Claude Code settings.
- **Verification Target**:
  - `go test -v ./internal/agy/... ./internal/claude/...` passes.
  - `harnez apply` (or running against a mock config) emits valid JSON matching `hooks.json` specifications.

---

### Milestone 5: Analytical Reporting in `harnez stats`
- **Goal**: Display actual tokens, measured savings, and potential opportunity savings in terminal reports and JSON output.
- **Scope**:
  - `internal/telemetry/query.go`: Update `AggregateByTool` to compute average actual tokens, total measured savings, and total potential savings.
  - `cmd/harnez/stats.go`: Render `AVG TOKENS`, `MEASURED SAVINGS`, and `POTENTIAL SAVINGS` columns in the terminal table.
  - Support `--json` emitting all newly aggregated metrics.
- **Verification Target**:
  - `go test -v ./cmd/harnez/stats_test.go` passes.
  - Running `harnez stats` renders a formatted table with new metrics and zero regression on existing output.

---

## 3. Sprint Success Criteria
1. All 5 milestones pass unit tests via `make test-q1` / `go test ./...`.
2. Existing tests and invariant guardrails remain 100% intact.
3. Smoke test with real `harnez hook` invocations verifies SQLite records are properly updated with `output_bytes`, `actual_tokens`, and `potential_savings_tokens`.
