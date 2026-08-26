# 065 — ConciseMode (Caveman Skill) & Command Output Distillation Hook (`harnez distill`)

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Observability & Token Efficiency
**Related**: [[039-agentic-loop-practices-and-sprint-command]], [[040-agent-context-duplication-and-file-read-discipline]], [[064-fresh-handoff-workflow-skill-and-friction-reporting]], `docs/practices/AgenticLoop.md`

---

## 1. Problem & Motivation

On local LLM setups (such as 27B models running on AMD Phoenix APU via Vulkan/MTP at ~6 tokens/second), output generation latency is the single largest bottleneck (166 ms per output token). When agents produce conversational pleasantries, verbose essay-like explanations, or repeat full file summaries, a single turn can take 60–90 seconds.

Concurrently, executing routine shell commands (`git status`, `go test ./...`, compiler runs, `ls -la`) dumps thousands of low-signal tokens into the model's context window, degrading the KV cache and accelerating context exhaustion.

To maximize agent throughput, we need:
1. **ConciseMode / Caveman Skill**: A behavioral rule that strips conversational fluff, pleasantries, and hedging, enforcing telegraphic high-signal output without altering code syntax.
2. **Command Output Distillation (`harnez distill`)**: A fast filter that compresses noisy tool outputs (stripping passing tests, deduplicating warnings, collapsing directory trees, and preserving failure traces) before feeding context to the model.

---

## 2. Technical Specification

### 2.1 ConciseMode Evergreen Doc & Template Ingestion (3 Graded Levels)

Create `docs/practices/ConciseMode.md` (managed by harnez) defining 3 distinct, selectable tiers of terseness:

1. **Level 1 — Concise Lite (Professional Terse)**:
   - Eliminates conversational pleasantries, opening fluff (*"Certainly!", "I'll be happy to help..."*), and speculative concluding remarks.
   - Retains full standard English grammar and complete sentence explanations.
   - Ideal for interactive user pairing.
2. **Level 2 — Concise Standard (Telegraphic / Core Caveman)**:
   - Strips grammatical filler, articles, and unnecessary connective phrases.
   - Uses structured high-density bullet fragments: `Action -> Finding -> Patch`.
   - Ideal for autonomous subagents and fast canary benchmark sweeps.
3. **Level 3 — Concise Ultra (Extreme Shorthand / Zero-Fluff)**:
   - Output strictly confined to essential diffs, command invocations, and single-line status confirmations (e.g. `PASS: 14 tests, built bin/app`).
   - Zero narrative text. Maximizes token efficiency on resource-constrained or low-TPS local hardware.

**Core Invariant Across All Levels**: Full code, diffs, tool parameters, and command syntax are preserved 100% verbatim.

### 2.2 Command Output Distillation (`harnez distill`)

- Implement `harnez distill [command...]`:
  - Wraps command execution or streams stdin.
  - Distills test runners (`go test`, `pytest`, `cargo test`) to only output failure traces, errors, and final summary.
  - Compresses `git status` / `git diff` noise.
  - Collapses duplicate lines (`[x12] warning: ...`).
  - Limits runaway terminal outputs with head/tail preservation (`[... 142 lines omitted ...]`).

---

## 3. Implementation & Verification Plan

1. Document `docs/practices/ConciseMode.md` and update `config.yaml` / `embed.go`.
2. Implement `cmd/distill.go` in `harnez`.
3. Add unit tests in `internal/` for distillation filters.
4. Verify with `harnez status` and run canary benchmarks in `lmcoder`.
