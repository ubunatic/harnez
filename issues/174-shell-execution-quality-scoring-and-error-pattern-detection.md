# 174 — Shell Execution Quality Scoring and Language Error Pattern Detection

**Status**: Closed — resolved in synthetic quality score classifier
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Telemetry / Quality Analysis
**Related**: [[116-tool-telemetry-schema-and-storage-layer]], [[118-harnez-exec-shell-interceptor]], [[120-harnez-stats-analytical-reporting]], `cmd/harnez/exec.go`, `internal/telemetry/`

---

## 1. Problem & Motivation

Over 1,180 shell calls have been recorded via `harnez exec`, but currently all shell executions have unpopulated (`NULL`) quality scores in `tool_catalog.sqlite`.

Relying solely on process exit codes is insufficient:
- Exit code 0 can sometimes still contain runtime warnings or ignored failures.
- Non-zero exit codes range from expected test failures to severe runtime panics, segfaults, or syntax errors.

To provide automated telemetry scoring without requiring manual user ratings, `harnez exec` should analyze command output for language-specific error signatures and assign an automated quality score (1–5).

## 2. Technical Specification

Implement an automated classifier in `cmd/harnez/exec.go` (or `internal/telemetry/classifier.go`) that inspects command stdout/stderr and exit codes to derive a synthetic rating:

### Scoring Criteria & Pattern Matching

1. **Go Error Patterns (Primary Focus)**:
   - **Score 1 (Critical Failure / Crash)**:
     - `panic: runtime error`
     - `fatal error:` / goroutine stack dumps
     - `SIGSEGV` / `SIGBUS` segmentation faults
   - **Score 2 (Build / Type Failure)**:
     - Go compiler errors (`undefined:`, `cannot use ... as type`, `syntax error:`, `imported and not used`)
   - **Score 3 (Test Failures / Lint Violations)**:
     - `--- FAIL:` in `go test` output, `golangci-lint` issue reports
   - **Score 5 (Clean Success)**:
     - Exit code 0 with no detected crash or diagnostic errors.

2. **Secondary Language Patterns**:
   - **Python**:
     - `Traceback (most recent call last):`, `SyntaxError:`, `Uncaught exception` $\rightarrow$ Score 1–2
   - **JavaScript / Node**:
     - `UnhandledPromiseRejection`, `TypeError:`, `ReferenceError:` $\rightarrow$ Score 1–2
   - **Generic Shell**:
     - `command not found`, `Permission denied` $\rightarrow$ Score 1

3. **Telemetry DB Update**:
   - Record the computed synthetic `score` in `tool_calls.score` when `call_type = 'shell'`.
   - Optionally record a failure category tag or reason for low scores.

## 3. Verification Plan

- [x] Unit-test output classification against fixture outputs:
   - Go runtime panic, Go compiler compile error, passing Go test, failing Go test.
   - Python traceback, Node unhandled promise rejection.
- [x] Execute test commands via `harnez exec` and verify `tool_catalog.sqlite` reflects accurate scores (1–5).
- [x] Run `harnez stats` and verify `Bash` tool call average scores reflect synthesized execution quality.
