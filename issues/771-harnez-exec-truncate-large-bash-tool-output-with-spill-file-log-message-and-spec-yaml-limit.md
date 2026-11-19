# 771 — harnez exec: truncate large bash tool output with spill file log message and spec yaml limit

**Status**: Open
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

## Decision

- **Spec & defaults (`spec/exec.yaml` + `spec/schemas/exec.schema.json`)**: Store `truncate: {enabled: true, max_output_bytes: 51200, max_output_lines: 2000, spill_dir: "", spill_prefix: "harnez-exec-"}` in `spec/exec.yaml` validated against `spec/schemas/exec.schema.json` and loaded via `internal/exec/spec.go` (zero Go defaults per `docs/Spec.md`). Document `spec/exec.yaml` in `docs/Spec.md`. (Host `/decide` + Consult `consult-771`, 2026-11-19.)
- **Live head-capped writer & lazy spill**: Use a mutex-guarded live head-capped writer sharing one byte+line budget and one lazy spill file across stdout and stderr (retaining emitted head in memory and creating the spill file only on first overflow; upstream `headTailBuffer` `capturedOutput` and `quotaLogFile` stay full/unmodified so scoring and Quota-1 failure extraction remain intact). (Host `/decide` + Consult `consult-771`, 2026-11-19.)
- **TTY, overrides, `--distill`, `--quota-1`, telemetry**:
  - Enable by default when stdout is NOT a TTY (`!term.IsTerminal`); skip on TTY unless forced via `HARNEZ_EXEC_TRUNCATE=1` or explicit `--max-output-bytes > 0`.
  - Precedence: `--max-output-bytes` flag (`0` disables) > `HARNEZ_EXEC_MAX_OUTPUT_BYTES` / `HARNEZ_EXEC_TRUNCATE` (including leading `VAR=val` command prefixes) > `spec/exec.yaml`.
  - On `--distill`, also apply the cap to the distilled output printed after exit (spilling full distilled output if capped).
  - On `--quota-1`, reuse `quotaLogPath` when `exitCode != 0` (avoiding duplicate multi-MB files) and a normal spill file when `exitCode == 0`.
  - Record shown byte count in `ToolCall.OutputBytes`; keep `RawBytes` and `--distill`-only `DistilledBytes` intact. (Host `/decide` + Consult `consult-771`, 2026-11-19.)

## M1 — `harnez exec` spill-file output truncation

- Write set (expected): `spec/exec.yaml`, `spec/schemas/exec.schema.json`, `internal/exec/spec.go`, `internal/exec/spec_test.go`, `internal/exec/exec.go` (+ `truncate.go` helper if cleaner), `internal/exec/exec_test.go`, `cmd/harnez/exec.go`, `cmd/harnez/exec_command.go`, `cmd/harnez/exec_test.go`, `cmd/harnez/testdata/help.golden`, `docs/Spec.md`, `docs/HookRewritePattern.md` (fix `:217-218` stale output claim if touched).
- Spill notice format when truncated (appended to `out` at exit):
  `[Full output: <spill_file_path>. Truncated: <lines_shown> lines shown (<limit> limit)]`
  where `<limit>` formats KB cleanly like `50.0KB` when `maxBytes` applies (or `<N> lines` if capped by line limit first; keep clear and tested).
- Note for `TestRunExecWrapper_BoundsCapturedOutputAndRetainsTail` (`internal/exec/exec_test.go:1646-1685`): opt out via `HARNEZ_EXEC_TRUNCATE=0` to keep its existing untruncated 3MB passthrough invariant intact AND add dedicated truncation tests rather than weakening existing assertions!
- Locked tests:
  1. Spec schema test (`internal/exec/spec_test.go`): valid `spec/exec.yaml` loads `51200`/`2000`, invalid schema rejects bad fields/types.
  2. Below-limit output: unchanged stdout/stderr, NO spill file created, no banner, `OutputBytes == RawBytes`.
  3. Above-limit byte & line caps (covering mixed stdout+stderr): stdout+stderr truncated cleanly at cap, exact banner printed with existing spill file path containing 100% verbatim full output, exit code preserved (`!= 0` stays `!= 0`), `RawBytes >= fullLen`, `OutputBytes <= cap`.
  4. Precedence & overrides: `--max-output-bytes` beats `HARNEZ_EXEC_MAX_OUTPUT_BYTES` beats spec; `HARNEZ_EXEC_TRUNCATE=0` and `--max-output-bytes=0` disable truncation; `HARNEZ_EXEC_TRUNCATE=1` forces truncation.
  5. `--distill` large output + `--quota-1` (failure reuses `quotaLogPath`, pass uses spill file + preserves `quotaLogFile` cleanup) + TTY vs non-TTY detection.
  6. `help.golden` updated and passes.
