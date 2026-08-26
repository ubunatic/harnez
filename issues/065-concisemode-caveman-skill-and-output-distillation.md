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

### 2.1 ConciseMode Evergreen Doc & Template Ingestion

- Create `docs/practices/ConciseMode.md` (managed by harnez).
- Update `harnez init` template to inject terse output conventions into `AGENTS.md` / `CLAUDE.md`:
  - Zero filler/pleasantries (*"I will now...", "Certainly!"*).
  - Telegraphic status: `Action -> Finding -> Patch`.
  - Full code, diff, and command syntax preserved verbatim.

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
