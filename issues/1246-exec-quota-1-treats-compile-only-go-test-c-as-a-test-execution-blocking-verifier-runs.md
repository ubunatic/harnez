# 1246 — exec: Quota-1 treats compile-only go test -c as a test execution, blocking verifier runs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 944, `docs/studies/2026-11-19-agy-native-swarm-vs-harnez-lean-sprint-cost-and-quality.md`, `internal/exec/quota1_command.go` (recheck live code before starting)

---

/goal Exempt compile-only `go test -c` invocations from Quota-1 test-execution counting in `harnez exec` so authors can compile-check test packages without consuming the host or verifier's single `make test-q1` allowance, while continuing to count real test executions (`go test`, `-run`, `-coverprofile`, `-exec`, compound invocations per #944), or stop and report when blocked on a user decision or denied permission. On a loop, hard blocker, or no progress, clear the goal and record a failure report with the resume point.

## 1. Problem & Motivation
Observed live during the 2026-11-19 native AGY `/swarm` run on issues `#1192` and `#1236`:
- Author workers (`release-1192` and `telemetry-1236`) compile-checked their packages with `GOWORK=off go test -c -o /dev/null ./internal/release` and `GOWORK=off go test -c -o /dev/null ./internal/telemetry` so the `verifier` worker could execute the single repository `HTO=0 make test-q1` run.
- `harnez exec`'s Quota-1 detector counted `telemetry-1236`'s `go test -c` call (`2026-11-19T11:43:12Z`) as a test execution, causing `verifier`'s subsequent `HTO=0 make test-q1` to fail with:
  `Quota-1: test execution blocked because no repository source files have been modified since the last test run (2026-11-19T11:43:12Z).`

## 2. Technical Specification / Findings
- Issue #944 added detection for targeted/coverage `go test` runs so agents do not bypass Quota-1 with `go test -run ...`.
- `-c` instructs `go test` to compile the test binary only without running any tests (`go help testflag`), making it equivalent in side effects to `go build` / `go vet` rather than test execution.
- Re-check `internal/exec/quota1_command.go` and `internal/quota1/` against `HEAD` before implementation to ensure compound commands (`go test -c ... && go test ...`), `--`, or flag combinations are handled safely without opening a Quota-1 bypass.

## 3. Implementation & Verification Plan
- Check live `internal/exec/quota1_command.go` and `spec/` first.
- Treat a `go test` command segment with `-c` as non-test-executing under Quota-1 while still counting compound invocations where another segment runs tests.
- Add unit tests covering `go test -c -o /dev/null ./...` (allowed without burning Quota-1), `go test ./...` / `go test -run Foo` (counted), and compound `go test -c ... && go test ...` (counted).
- Verify with `HTO=0 make lint` and `HTO=0 make test-q1`.
