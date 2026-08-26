# 066 — Native Go Command Output Distillation (`harnez distill`) Architecture

**Status**: Closed — resolved
**Priority**: P1 (High)
**Severity**: Major
**Category**: Observability & Token Efficiency
**Related**: [[065-concisemode-caveman-skill-and-output-distillation]], `docs/practices/AgenticLoop.md`

---

## 1. Problem & Motivation

Routine terminal commands (`go test ./...`, `git status`, `git diff`, build logs, compiler diagnostics) dump thousands of low-signal tokens into agent context windows. This wastes KV cache capacity and degrades local LLM throughput.

While tools like RTK (Rust Token Killer) exist in the Rust ecosystem, we want a **native Go implementation** in `harnez` (`harnez distill`) that adopts the core algorithms of RTK (smart test filtering, line deduplication, ANSI stripping, and head/tail preservation) while maintaining a zero-dependency pure Go codebase.

---

## 2. Technical Specification

### 2.1 Core Distillation Filters (`internal/distill/`)

1. **`FilterGoTest(r io.Reader) string`**:
   - Omits passing tests (`=== RUN`, `--- PASS`).
   - Retains failure blocks (`--- FAIL`, stack traces, panics, compiler errors).
   - Preserves package outcome lines (`ok ...`, `FAIL ...`).
2. **`FilterGit(r io.Reader) string`**:
   - Condenses status tables and strips verbose untracked file listings.
3. **`FilterDeduplicate(r io.Reader) string`**:
   - Collapses identical sequential or frequent warning lines into `[xN] <message>`.
4. **`FilterHeadTail(lines []string, maxLines int) string`**:
   - Truncates oversized outputs preserving the first $N/2$ lines and last $N/2$ lines with an explicit omission count `[... X lines omitted ...]`.
5. **`StripANSI(s string) string`**:
   - Removes terminal colors and cursor escape sequences.

### 2.2 CLI Command (`harnez distill`)

```bash
# Streaming mode (stdin)
go test ./... | harnez distill

# Wrapper mode
harnez distill -- go test ./...
harnez distill -- git status
```

---

## 3. Implementation & Verification Plan

1. Implement `internal/distill/` with comprehensive unit tests for each filter mode.
2. Add `cmd/distill.go` in `harnez`.
3. Add `harnez distill` benchmark tests comparing raw vs distilled token counts.
4. Verify with `harnez status` and mark closed.
