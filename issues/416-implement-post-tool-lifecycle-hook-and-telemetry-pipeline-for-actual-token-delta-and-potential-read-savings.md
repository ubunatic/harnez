# 416 — Implement post-tool lifecycle hook and telemetry pipeline for actual token delta and potential read savings

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Hooks & Telemetry / Observability
**Related**: #405, #412, #296, #173, #120, #142

---

## 1. Problem Statement & Motivation

Currently, Harnez relies exclusively on `PreToolUse` hooks (such as `harnez hook agy` and `harnez guard`) to intercept and observe agent tool invocations before execution. While this permits gating and basic call logging into `tool_catalog.sqlite`, it leaves significant observability gaps:

1. **No Post-Execution Visibility**: Once a tool executes, Harnez has no `PostToolUse` or `PostInvocation` hook listening to inspect tool output payloads, execution status, or response sizes.
2. **Missing Ground-Truth Token Telemetry**: Real token costs for individual tool calls (including `harnez read -I`, `view_file`, `grep_search`, `run_command`) are neither measured nor stored in telemetry. `harnez stats` only reports text distillation byte savings from `harnez distill`, leaving vision and tool token deltas unrecorded.
3. **No Native Tool Opportunity Cost Tracking**: When agents invoke native file-reading tools (e.g. `view_file`, `ReadMultipleFiles`, `cat`), Harnez does not quantify the *potential* token savings that would have been achieved had the agent used `harnez read -I` or `harnez read -L`.

To evaluate agent tool efficiency and build data-driven policies for redirecting agents to optimized Harnez tools, Harnez needs a post-tool hook chain and telemetry pipeline that records real token deltas and potential savings.

---

## 2. Technical Scope & Architecture

### 2.1 PostToolUse & PostInvocation Hook Manifests
- **AGY / Antigravity**: Update `internal/agy/hooks.go` (`BuildHooksDoc`) to register `PostToolUse` (and `PostInvocation` where appropriate) targeting `*` or specific tool matchers, invoking `harnez hook post-tool` (or `harnez hook agy-post`).
- **Claude Code**: Update `internal/claude/hooks.go` / `settings.json` to register `PostToolUse` hook handlers.
- **Codex**: Wire corresponding post-tool notification hooks where available.

### 2.2 Correlation & Step Tracking
- Correlation key: `(session_id, stepIdx)` (with fallback to conversation timestamp / tool ID).
- **PreToolUse (`step N`)**:
  - Record initiation timestamp $t_0$, `tool_name`, args, and snapshot pre-call session/transcript token or byte offset $S_0$.
- **PostToolUse (`step N`)**:
  - Correlate with `(session_id, stepIdx)`.
  - Read tool output payload from `transcript.jsonl` / stdin payload (bytes, lines, error status).
  - Record execution duration $\Delta t = t_1 - t_0$.

### 2.3 Actual Token Delta ($\Delta \text{Tokens}$) Measurement
- **Turn-by-Turn Delta Attribution**:
  - Measure the delta in billed session tokens / input tokens before and after the tool execution:
    $$\Delta \text{InputTokens} = \text{SessionTokens}_{\text{turn } N+1} - \text{SessionTokens}_{\text{turn } N}$$
  - Captures the complete real payload (tool output text / image tiles + prompt scaffolding + model thinking/noise).
- **Tool Output Tokenization**:
  - For text tools (e.g. `view_file`, `run_command` output): compute exact text tokens via tokenizer.
  - For visual image cards (e.g. `harnez read -I` output): compute exact provider-specific ViT tile tokens from image dimensions.

### 2.4 Potential Savings Computation for Native Tools
- For watched native file-reading tools (e.g. `view_file`, `ReadMultipleFiles`, `cat`):
  - Ingest the returned file content payload.
  - Calculate actual tokens consumed by the native text representation.
  - Compute hypothetical ViT tokens for the equivalent `harnez read -I` visual card (and `harnez read -L` line slice).
  - Compute `potential_token_savings = actual_text_tokens - hypothetical_vit_tokens`.
  - Record `potential_savings_bytes` and `potential_savings_tokens` in telemetry.

### 2.5 Schema & Database Migration
- Extend `tool_calls` table in `internal/telemetry/schema.go`:
  - `actual_tokens INTEGER` (actual measured tokens consumed by this tool call / turn delta).
  - `potential_savings_tokens INTEGER` (hypothetical token savings if optimized tool was used).
  - `potential_savings_bytes INTEGER` (hypothetical byte savings).
  - `output_bytes INTEGER` (raw bytes returned by tool result).
- Add indices and migration handling for existing SQLite databases.

### 2.6 Analytical Reporting in `harnez stats`
- Update `harnez stats` to report:
  - Average actual token cost per tool.
  - Cumulative measured token savings from `harnez read -I` vs native `view_file`.
  - Cumulative *opportunity loss / potential savings* from unredirected native tool calls.

---

## 3. Implementation Checklist

- [ ] Register `PostToolUse` and `PostInvocation` in `internal/agy/hooks.go` and `internal/claude/hooks.go`.
- [ ] Implement `harnez hook post-tool` handler in `cmd/harnez/hook.go` with `(session_id, stepIdx)` correlation.
- [ ] Add transcript step extractor for reading tool results and media attachments from `transcript.jsonl`.
- [ ] Implement potential savings evaluator (tokenizing native `view_file` payloads vs `readcard` ViT estimates).
- [ ] Update `internal/telemetry/schema.go`, `insert.go`, and `query.go` to store and query token deltas and potential savings.
- [ ] Add analytical reporting in `cmd/harnez/stats.go` for actual token usage and potential savings breakdown.
- [ ] Add unit tests in `internal/telemetry/` and `cmd/harnez/` verifying post-hook correlation and stats aggregation.
