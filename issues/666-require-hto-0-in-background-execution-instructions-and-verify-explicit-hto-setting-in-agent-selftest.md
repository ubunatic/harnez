# 666 — Require HTO=0 in background execution instructions and verify explicit HTO setting in agent selftest

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 520, 547, 630, 665; [agent selftest](../cmd/harnez/agent_selftest.go), [exec](../cmd/harnez/exec.go), [Bash doc](../docs/lang/Bash.md), [AgenticLoop](../docs/practices/AgenticLoop.md), [Tools rule](../.harnez/rules/Tools.md)

---

## 1. Problem & Motivation

When agents dispatch commands or test suites in the background as a host Task/Job (such as `run_command` in background, Claude Code `run_in_background: true`, or detached wait wrappers), commands run under `harnez exec`'s default 60-second execution window (`defaultExecTimeout = 60s`).
Without `HTO=0` (or an explicit timeout override), any background command exceeding 60s is abruptly killed with `SIGKILL` (`exit status 137`).

To prevent premature background task kills and eliminate polling/sleep anti-patterns:
1. General instructions for running Bash and executing background tasks/jobs must explicitly mandate `HTO=0` (e.g., `HTO=0 make test-q1`, `HTO=0 harnez ...`).
2. The `/harnez-selftest` command must record the `HTO` environment variable when `--step background` is invoked and verify in `--step verify` that `HTO` was explicitly set (e.g. `HTO=0` or other non-empty value), failing with an actionable diagnostic if omitted.

## 2. Technical Specification & Design

1. **Instruction Updates**:
   - Update `.harnez/rules/Tools.md` and copyable documentation (`docs/lang/Bash.md` / `docs/practices/AgenticLoop.md`) to make the `HTO=0` requirement prominent for background tasks and long-running job invocations.
   - Emphasize that agents must rely purely on reactive system completion notifications instead of polling or issuing `sleep` commands.

2. **Self-Test State & Verification (`cmd/harnez/agent_selftest.go`)**:
   - In `--step background`: record whether `HTO` was set in the environment (`os.Getenv("HTO")`) in the selftest state file (`BackgroundHTO`).
   - In `--step verify`: check that `BackgroundHTO` is non-empty (e.g. `"0"` or explicitly defined). If `BackgroundHTO == ""`, fail verification with:
     `--step background was invoked without setting HTO (e.g., HTO=0 harnez agent selftest --step background); background tasks must explicitly set HTO=0 to avoid 60s timeout kills`

3. **Test Coverage (`cmd/harnez/agent_selftest_test.go`)**:
   - Update full-sequence success tests to pass `HTO=0`.
   - Add test case verifying that `verify` fails with a clear diagnostic when `HTO` is not set during `--step background`.

## 3. Implementation & Verification Plan

- **/goal**: `/goal Document the HTO=0 requirement across general bash/task instructions and enforce explicit HTO setting in harnez agent selftest background and verify steps, backed by automated tests; stop and report when blocked on a user decision or denied permission.`
- Update `cmd/harnez/agent_selftest.go` to capture `BackgroundHTO` and validate in `verify`.
- Update `cmd/harnez/agent_selftest_test.go` for `HTO` validation.
- Update docs and rule files (`docs/lang/Bash.md`, `.harnez/rules/Tools.md`, `docs/practices/AgenticLoop.md`).
- Run `make test-q1` to verify all tests pass.
