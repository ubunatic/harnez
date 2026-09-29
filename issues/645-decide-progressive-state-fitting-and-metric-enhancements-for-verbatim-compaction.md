# 645 — decide: progressive state fitting and metric enhancements for verbatim compaction

**Status**: Closed — implemented progressive state fitting, reproducible question prompts, exact truncation notices, and byte reduction metrics
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: issues/635, issues/633, docs/practices/AgenticLoop.md

---

## 1. Problem & Motivation
Following research and battle-tested patterns from `fast-jev-compaction`:
1. Decision request context can exceed provider model budgets (e.g. 25k–30k tokens) during long sessions if messages and tool states are attached naively without progressive fitting.
2. Question phrasing should emphasize reproducibility: knowing the call was made matters for future steps, and whether re-running the tool is acceptable.
3. Compaction outputs and truncation markers should communicate the exact character/token reduction to the model (`[harnez compact truncated N chars...]`).
4. Compaction reports and metrics need byte counts and reduction ratios (`ReductionRatio`) to support automated compaction gating and quality telemetry in 100% pure Go.

## 2. Technical Specification / Findings
1. **Refined Decision Prompts**:
   - `keep_call`: *"Tool call <id> (<tool>) should stay in the history: knowing this call was made, with its input, still matters for what the assistant does next"*.
   - `keep_result`: *"The full output of tool call <id> (<tool>, <N> bytes) should stay in the history verbatim: the assistant still needs its contents and re-running the tool would not do"*.
2. **Progressive State Fitting (`fitState`)**:
   - Construct history entries where past tool results are summarized (e.g. `ok, %d chars (omitted)`).
   - Iteratively fit history and candidate tool inputs into a configured token/byte budget (default ~25k tokens / 100KB) across staged reductions (full inputs -> 200 chars -> 60 chars -> abridged message texts).
3. **Truncation Notice**:
   - Format: `[harnez compact truncated %d chars of this tool result; re-run the tool if needed]`.
4. **Byte & Reduction Metrics in `CompactReport`**:
   - `OriginalBytes int64`
   - `CompactedBytes int64`
   - `ReductionRatio float64`
   - Render in CLI `--format summary` and `--format json`.

## 3. Implementation & Verification Plan
1. Enhance `internal/decide/compact.go` with state fitting, refined questions, exact truncation notices, and byte metrics.
2. Update `cmd/harnez/compact.go` summary output and flag support.
3. Add unit test coverage in `internal/decide/compact_test.go` and `cmd/harnez/compact_test.go`.
4. Run `make test-q1` and request review from `terra:med`.

