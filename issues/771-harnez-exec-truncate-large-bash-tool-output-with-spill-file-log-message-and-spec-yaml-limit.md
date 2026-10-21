# 771 — harnez exec: truncate large bash tool output with spill file log message and spec yaml limit

**Status**: Backlog — grooming pass 843 M4 (2026-10-22); promote when needed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: `cmd/harnez/exec.go`, `spec/actions.yaml`, `issues/118`, `issues/182`

---

## 1. Problem & Motivation
When agents run shell commands producing large volumes of stdout/stderr, unconstrained terminal streams flood the context window, burning through context quotas and inducing LLM context degradation.

Tools like the "pi" agent handle this cleanly by truncating output streams at a configurable size boundary, preserving the full output in a temporary spill file, and returning a concise reference notice in the output stream:
```
 [Full output: /tmp/pi-bash-219615c011d7a97c.log. Truncated: 344 lines shown (50.0KB limit)]
```

`harnez exec` (the execution interceptor and hook wrapper for bash tool calls) should support similar output truncation with spill-to-file and explicit notification messages, with the limit configurable via `spec/` YAML / configuration.

## 2. Technical Specification / Findings
- **Spill & Truncation Mechanism**:
  - When `harnez exec` runs a command whose output exceeds a specified threshold (e.g. byte or line limit):
    - The full un-truncated output is streamed to a temporary log file (e.g. `/tmp/harnez-exec-<id>.log` or `~/.local/state/harnez/logs/exec-<id>.log`).
    - The output returned to the host/agent stream is capped at the configured limit (e.g., 50 KB / $N$ lines).
    - A truncation banner is appended or prepended matching the format:
      ```
       [Full output: <spill_file_path>. Truncated: <lines_shown> lines shown (<limit_kb>KB limit)]
      ```
- **Spec / YAML Configuration**:
  - Store default truncation thresholds in `spec/` (e.g. `spec/actions.yaml` or a dedicated exec section) so Go code does not hardcode arbitrary size limits.
  - Allow override via environment variables (e.g. `HARNEZ_EXEC_MAX_OUTPUT_BYTES`, `HARNEZ_EXEC_TRUNCATE=1`) or CLI flags (`--max-output-bytes`).
  - Distillation interaction: coordinates with `harnez distill` and Quota-1 logging so failure extraction still inspects the required head/tail buffers or the full spill log.

## 3. Implementation & Verification Plan
- **Goal**: `/goal Implement output truncation with spill file logging in harnez exec, add spec/ yaml configuration for limit thresholds, and verify with unit tests, or stop and report when blocked on user input or denied permission.`
- **Implementation**:
  - Define truncation thresholds in `spec/` YAML and load them into `cmd/harnez/exec.go` / `execOptions`.
  - Implement a spill-file writer and stream truncator in `runExecWrapper`.
  - Format truncation notice: `[Full output: <path>. Truncated: <n> lines shown (<limit> limit)]`.
  - Ensure exit codes, telemetry recording (`rawBytes`, `distilledBytes`), and Quota-1 log extraction remain accurate.
- **Verification**:
  - Author unit tests in `cmd/harnez/exec_test.go` with commands producing output above and below the threshold.
  - Verify that the spill file contains full output while the returned reader contains truncated output with the exact banner message.
  - Test with `make test-q1` / `go test ./cmd/harnez`.
