# 555 — quota-1 failure summary shows the real error when there are no FAIL lines

**Status**: Open
**Priority**: P2
**Severity**: Low
**Category**: Feature / Exec
**Related**: [[554-sandbox-quota-1-test-runs]]

---

## Problem

When a quota-1 run fails before any test runs (`go vet`, compile error), `quota1FailureSummary`
(`cmd/harnez/exec.go`) prints only "Quota-1 command failed; no '--- FAIL' lines were found in
output." and the log path. The agent must then read the log to find the error. Seen in agy session
35ecec0a (loom, 2026-09-24): two runs failed on `"fmt" imported and not used` in a new test file, and
the summary did not show it.

## /goal

With no `--- FAIL` lines, the summary shows the error lines from the output (e.g. vet/compile lines
`file.go:N:M: ...`, `FAIL <pkg> [build failed]`, `panic:`), capped at a few lines; if none match,
the last ~10 lines of output. Unit tests cover vet error, build failure, panic and unmatched output.

## Notes

- Logs are written only on failure (`writeQuota1FailureLog`); a passing run has no log. Intended.
- Idea, not in scope: run `go vet` before the one allowed test run so a vet error does not use it up.
