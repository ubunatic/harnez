# 1288 — Slow TestAgentSelftestGeneratedSessionAcrossShells takes 31s

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Performance
**Related**: docs/TestTiers.md

---

## 1. Problem & Motivation
`TestAgentSelftestGeneratedSessionAcrossShells` (cmd/harnez) is the slowest test in the package. In a full run it takes about 31.5s. The package runs about 120s, and its top-level tests add up to about 119s serially. The slow test makes `go test ./cmd/harnez` close to any package timeout a developer sets, such as the 120s used in ad-hoc runs. The cost is paid on every full run.

## 2. Technical Specification / Findings
- Measured: 31.5s in a full `go test -json ./...` run with TMPDIR unset (harnez 7cba5775 tree), the top entry in the cmd/harnez per-test list. The next slowest tests take about 4s.
- The test builds the whole CLI inside the test with `go build -o <tmp>/harnez .` and then runs generated shell sessions. It also sets a 50s context timeout.
- Likely cause, not yet measured: the `go build` step. Its compile cost is charged to every run and can't be cached by the test cache.
- Shell-session time may be the other share; it has not been split out.

## 3. Implementation & Verification Plan
1. Time the test with and without the `go build` step (`go test -run TestAgentSelftestGeneratedSessionAcrossShells -v`, then add timing around the build).
2. If the build dominates, build the test binary once per package (TestMain or a sync.Once) and reuse it, or point the test at the already-installed harnez when present.
3. Keep the test's shell coverage: generated sessions across shells must still be exercised.
4. Verify: the test passes, and its elapsed time drops below 5s on a warm cache. Confirm the cmd/harnez package time falls by about the same amount.
