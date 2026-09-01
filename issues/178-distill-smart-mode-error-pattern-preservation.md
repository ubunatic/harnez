# 178 — Distill: Opt-In Smart Mode with Classifier-Driven Error Pattern Preservation

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Distillation / Ergonomics
**Related**: [[066-native-go-command-output-distillation]], [[174-shell-execution-quality-scoring-and-error-pattern-detection]], `internal/distill/distill.go`, `internal/telemetry/score.go`, `cmd/harnez/distill.go`

---

## 1. Problem & Motivation

`harnez distill` currently supports filtering modes: `auto`, `gotest`, `git`, and `raw`.
In `auto` and `raw` modes, distillation performs static noise reduction (ANSI stripping, passing test truncation, repeated line collapsing).

However, when commands fail with complex errors (such as Go runtime panics, multi-goroutine stack traces, Python tracebacks, or compiler type-mismatch blocks), standard head/tail line-capping or generic deduping risks truncating the exact stack frame or root-cause line the agent needs to diagnose the failure.

In ticket 174, we introduced [`ScoreShell`](../internal/telemetry/score.go), a pure library function capable of identifying:
- Go panics / fatal runtime errors / goroutine stack dumps
- Go compiler / type / syntax errors
- Go test failures / assertion errors
- Python tracebacks & Node unhandled promise rejections

Reusing this classifier inside `distill` can enable a **Smart / Adaptive Distillation** mode that dynamically preserves high-value error context while aggressively pruning irrelevant output.

## 2. Technical Specification

### 1. Opt-In Behavior & Mode Definition
The default distillation mode (`auto`) must remain simple and predictable (no surprising dynamic omissions).
Introduce a dedicated opt-in mode:
- **CLI Flag / Mode**: `harnez distill --mode smart` (or `harnez distill --smart`)
- **Type**: `distill.ModeSmart Mode = "smart"` in `internal/distill/distill.go`

### 2. Error-Aware Distillation Pipeline
When `ModeSmart` is active:
1. **Error Scan**: Run `telemetry.ScoreShell(output, exitCode)` to classify output quality (1–5) and identify error signatures.
2. **Adaptive Filtering Rules**:
   - **On Panic / Fatal Crash (Score 1)**:
     - Detect and fully preserve the crashing goroutine stack trace, panic header, and panic location.
     - Prune background idle goroutines (`goroutine [chan receive]`, `goroutine [select]`, etc.) to minimize token bloat without losing the offending crash trace.
   - **On Build / Syntax Error (Score 2)**:
     - Pin all compiler error lines and their immediate source context lines.
     - Drop noisy tool banner lines or irrelevant environment logs.
   - **On Test Failure (Score 3)**:
     - Run `FilterGoTest` while ensuring failure blocks and assertion diffs are never truncated by `MaxLines` thresholds.
   - **On Clean Success (Score 5)**:
     - Apply standard high-compression noise reduction (stripping ANSI, collapsing routine progress indicators).

### 3. CLI & Hook Integration
- Add `--mode smart` to `harnez distill`.
- Support configuring `distill.mode: smart` in `config.yaml` or harness hook definitions.

## 3. Constraints
- **Zero regression in default mode**: `mode=auto`, `mode=gotest`, `mode=git`, and `mode=raw` behavior must remain byte-for-byte unchanged.
- Smart mode must be purely deterministic and covered by fixture-based unit tests.

## 4. Verification Plan

1. Unit-test `ModeSmart` in `internal/distill/distill_test.go` with fixture outputs:
   - Multi-goroutine panic dump (verifies crash goroutine is kept while 50 idle goroutines are pruned).
   - Go compiler error with 200 lines of build noise (verifies compile error lines are pinned).
   - Python traceback in a large build log.
2. Verify `harnez distill --mode smart -- go test ./...` on failing and passing test suites.
3. Verify `make test` passes cleanly.
